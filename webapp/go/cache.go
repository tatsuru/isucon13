package main

import "fmt"

var supercache = make(map[string]interface{})

func setCache(key string, value int64) {
	supercache[key] = value
}

func getCache(key string) int64 {
	if supercache[key] == nil {
		return 0
	}
	return supercache[key].(int64)
}

func incrCache(key string) {
	currentValue := getCache(key)
	if currentValue == 0 {
		setCache(key, 1)
	} else {
		setCache(key, currentValue+1)
	}
}

func decrCache(key string) {
	currentValue := getCache(key)
	if currentValue != 0 {
		setCache(key, currentValue-1)
	}
}

func addCache(key string, value int64) {
	currentValue := getCache(key)
	setCache(key, currentValue+value)
}

func updateMaxValueIfNeeded(key string, value int64) {
	currentValue := getCache(key)
	if currentValue == 0 {
		setCache(key, value)
	}
}

type reactionsAndUser struct {
	ReacteeUserID  int64 `db:"reactee_user_id"`
	ReactionsCount int64 `db:"reactions_count"`
}

type reactionsAndLivestream struct {
	LivestreamID   int64 `db:"livestream_id"`
	ReactionsCount int64 `db:"reactions_count"`
}

type tipsAndUser struct {
	ReacteeUserID int64 `db:"reactee_user_id"`
	TipsCount     int64 `db:"tips_count"`
}

type tipsAndLivestream struct {
	LivestreamID int64 `db:"livestream_id"`
	TipsCount    int64 `db:"tips_count"`
}

type commentsAndLivestream struct {
	LivestreamID  int64 `db:"livestream_id"`
	CommentsCount int64 `db:"comments_count"`
}

type reportsAndLivesream struct {
	LivestreamID int64 `db:"livestream_id"`
	ReportsCount int64 `db:"reports_count"`
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
			GROUP BY reactee_user_id
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
			GROUP BY reactee_user_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate tips: %w", err)
	}
	for _, tipsAndUser := range tipsAndUsers {
		addCache(fmt.Sprintf("user:%d:tips", tipsAndUser.ReacteeUserID), tipsAndUser.TipsCount)
	}

	var tipsAndLivestreams []tipsAndLivestream
	err = dbConn.Select(
		&tipsAndLivestreams,
		`
			SELECT
				livestream_id,
				IFNULL(MAX(tip), 0) as tips_count
			FROM livecomments
			GROUP BY livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate tips: %w", err)
	}
	for _, tipsAndLivestream := range tipsAndLivestreams {
		updateMaxValueIfNeeded(fmt.Sprintf("livestream:%d:maxTip", tipsAndLivestream.LivestreamID), tipsAndLivestream.TipsCount)
	}

	var totalTipsAndLivestreams []tipsAndLivestream
	err = dbConn.Select(
		&totalTipsAndLivestreams,
		`
			SELECT
				livestream_id,
				SUM(tip) as tips_count
			FROM livecomments
			GROUP BY livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate tips: %w", err)
	}
	for _, tipsAndLivestream := range totalTipsAndLivestreams {
		setCache(fmt.Sprintf("livestream:%d:tips", tipsAndLivestream.LivestreamID), tipsAndLivestream.TipsCount)
	}

	var reactionsAndLivestreams []reactionsAndLivestream
	err = dbConn.Select(
		&reactionsAndLivestreams,
		`
			SELECT
				livestream_id,
				count(*) as reactions_count
			FROM reactions
			GROUP BY livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate reactions: %w", err)
	}
	for _, reactionsAndLivestream := range reactionsAndLivestreams {
		setCache(fmt.Sprintf("livestream:%d:reactions", reactionsAndLivestream.LivestreamID), reactionsAndLivestream.ReactionsCount)
	}

	var commentsAndLivestreams []commentsAndLivestream
	err = dbConn.Select(
		&commentsAndLivestreams,
		`
			SELECT
				livestream_id,
				count(*) as comments_count
			FROM livecomments
			GROUP BY livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate comments: %w", err)
	}
	for _, commentsAndLivestream := range commentsAndLivestreams {
		setCache(fmt.Sprintf("livestream:%d:comments", commentsAndLivestream.LivestreamID), commentsAndLivestream.CommentsCount)
	}

	var reportsAndLivestreams []reportsAndLivesream
	err = dbConn.Select(
		&reportsAndLivestreams,
		`
			SELECT
				livestream_id,
				count(*) as reports_count
			FROM livecomment_reports
			GROUP BY livestream_id
		`,
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate reports: %w", err)
	}
	for _, reportsAndLivestream := range reportsAndLivestreams {
		setCache(fmt.Sprintf("livestream:%d:reports", reportsAndLivestream.LivestreamID), reportsAndLivestream.ReportsCount)
	}

	return nil
}
