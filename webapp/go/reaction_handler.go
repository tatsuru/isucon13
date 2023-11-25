package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type ReactionModel struct {
	ID           int64  `db:"id"`
	EmojiName    string `db:"emoji_name"`
	UserID       int64  `db:"user_id"`
	LivestreamID int64  `db:"livestream_id"`
	CreatedAt    int64  `db:"created_at"`
}

type Reaction struct {
	ID         int64      `json:"id"`
	EmojiName  string     `json:"emoji_name"`
	User       User       `json:"user"`
	Livestream Livestream `json:"livestream"`
	CreatedAt  int64      `json:"created_at"`
}

type PostReactionRequest struct {
	EmojiName string `json:"emoji_name"`
}

func getReactionsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	query := "SELECT * FROM reactions WHERE livestream_id = ? ORDER BY created_at DESC"
	if c.QueryParam("limit") != "" {
		limit, err := strconv.Atoi(c.QueryParam("limit"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
		}
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	reactionModels := []ReactionModel{}
	if err := tx.SelectContext(ctx, &reactionModels, query, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "failed to get reactions")
	}

	// reactions := make([]Reaction, len(reactionModels))
	// for i := range reactionModels {
	// 	reaction, err := fillReactionResponse(ctx, tx, reactionModels[i])
	// 	if err != nil {
	// 		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	// 	}

	// 	reactions[i] = reaction
	// }

	reactions, err := fillReactionResponses(ctx, tx, reactionModels)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reactions: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, reactions)
}

func postReactionHandler(c echo.Context) error {
	ctx := c.Request().Context()
	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *PostReactionRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	reactionModel := ReactionModel{
		UserID:       int64(userID),
		LivestreamID: int64(livestreamID),
		EmojiName:    req.EmojiName,
		CreatedAt:    time.Now().Unix(),
	}

	result, err := tx.NamedExecContext(ctx, "INSERT INTO reactions (user_id, livestream_id, emoji_name, created_at) VALUES (:user_id, :livestream_id, :emoji_name, :created_at)", reactionModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert reaction: "+err.Error())
	}

	reactionID, err := result.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted reaction id: "+err.Error())
	}
	reactionModel.ID = reactionID

	reaction, err := fillReactionResponse(ctx, tx, reactionModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, reaction)
}

func fillReactionResponses(ctx context.Context, tx *sqlx.Tx, reactionModels []ReactionModel) ([]Reaction, error) {
	if len(reactionModels) == 0 {
		return []Reaction{}, nil
	}

	reactions := make([]Reaction, len(reactionModels))

	// populate users
	users := make(map[int64]User)

	userIDs := make([]int64, len(reactionModels))
	for i := range reactionModels {
		userIDs = append(userIDs, reactionModels[i].UserID)
	}

	query := "SELECT * FROM users WHERE id IN (?)"
	query, args, err := sqlx.In(query, userIDs)
	if err != nil {
		return nil, err
	}

	// TODO: ユーザーの数とリアクションの数が一致しない場合は返したほうがいいかも

	userModels := []UserModel{}
	if err := tx.SelectContext(ctx, &userModels, query, args...); err != nil {
		return nil, err
	}

	for user := range userModels {
		user, err := fillUserResponse(ctx, tx, userModels[user])
		if err != nil {
			return nil, err
		}

		var reactionModelID int64
		for i := range reactionModels {
			if reactionModels[i].UserID == user.ID {
				reactionModelID = reactionModels[i].ID
				break
			}
		}

		users[reactionModelID] = user
	}

	// populate liveStream
	livestreams := make(map[int64]Livestream)

	livestreamIDs := make([]int64, len(reactionModels))
	for i := range reactionModels {
		livestreamIDs = append(livestreamIDs, reactionModels[i].LivestreamID)
	}

	query = "SELECT * FROM livestreams WHERE id IN (?)"
	query, args, err = sqlx.In(query, livestreamIDs)
	if err != nil {
		return nil, err
	}

	// TODO: ライブの数とリアクションの数が一致しない場合は返したほうがいいかも

	livestreamModels := []LivestreamModel{}
	if err := tx.SelectContext(ctx, &livestreamModels, query, args...); err != nil {
		return nil, err
	}

	for livestream := range livestreamModels {
		livestream, err := fillLivestreamResponse(ctx, tx, livestreamModels[livestream])
		if err != nil {
			return nil, err
		}

		var reactionModelID int64
		for i := range reactionModels {
			if reactionModels[i].LivestreamID == livestream.ID {
				reactionModelID = reactionModels[i].ID
				break
			}
		}

		livestreams[reactionModelID] = livestream
	}

	for _, reactionModel := range reactionModels {
		reaction := Reaction{
			ID:         reactionModel.ID,
			EmojiName:  reactionModel.EmojiName,
			User:       users[reactionModel.ID],
			Livestream: livestreams[reactionModel.ID],
			CreatedAt:  reactionModel.CreatedAt,
		}

		reactions = append(reactions, reaction)
	}

	return reactions, nil
}

func fillReactionResponse(ctx context.Context, tx *sqlx.Tx, reactionModel ReactionModel) (Reaction, error) {
	userModel := UserModel{}
	if err := tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE id = ?", reactionModel.UserID); err != nil {
		return Reaction{}, err
	}
	user, err := fillUserResponse(ctx, tx, userModel)
	if err != nil {
		return Reaction{}, err
	}

	livestreamModel := LivestreamModel{}
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", reactionModel.LivestreamID); err != nil {
		return Reaction{}, err
	}
	livestream, err := fillLivestreamResponse(ctx, tx, livestreamModel)
	if err != nil {
		return Reaction{}, err
	}

	reaction := Reaction{
		ID:         reactionModel.ID,
		EmojiName:  reactionModel.EmojiName,
		User:       user,
		Livestream: livestream,
		CreatedAt:  reactionModel.CreatedAt,
	}

	return reaction, nil
}
