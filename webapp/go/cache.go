package main

import "fmt"

var supercache = make(map[string]interface{})

func setCache(key string, value interface{}) {
	supercache[key] = value
}

func getCache(key string) interface{} {
	return supercache[key]
}

func incrCache(key string) {
	currentValue := getCache(key)
	if currentValue == nil {
		setCache(key, 1)
	} else {
		setCache(key, currentValue.(int64)+1)
	}
}

func addCache(key string, value int64) {
	currentValue := getCache(key)
	if currentValue == nil {
		setCache(key, value)
	} else {
		setCache(key, currentValue.(int64)+value)
	}
}

type reactionsAndUser struct {
	ReacteeUserID  int64 `db:"reactee_user_id"`
	ReactionsCount int64 `db:"reactions_count"`
}

type tipsAndUser struct {
	ReacteeUserID int64 `db:"reactee_user_id"`
	TipsCount     int64 `db:"tips_count"`
}

func setupTipsAndReactionsCache() error {
	var tipsAndReactions []reactionsAndUser
	err := dbConn.Select(
		&tipsAndReactions,
		`
			SELECT
				livestreams.user_id as reactee_user_id,
				count(*) as reactions_count
			FROM livecomments
			INNER JOIN livestreams ON livestreams.id = livecomments.livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate reactions: %w", err)
	}

	for _, tipsAndReaction := range tipsAndReactions {
		addCache(fmt.Sprintf("user:%d:reactions", tipsAndReaction.ReacteeUserID), tipsAndReaction.ReactionsCount)
	}

	var tipsAndUsers []tipsAndUser
	err = dbConn.Select(
		&tipsAndUsers,
		`
			SELECT
				livestreams.user_id as reactee_user_id,
				sum(livecomments.tip) as tips_count
			FROM livecomments
			INNER JOIN livestreams ON livestreams.id = livecomments.livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate tips: %w", err)
	}
	for _, tipsAndUser := range tipsAndUsers {
		addCache(fmt.Sprintf("user:%d:tips", tipsAndUser.ReacteeUserID), tipsAndUser.TipsCount)
	}
}
