package db

import (
	"fifa2026/src/internal/models"
	"time"
)

// SaveTournament 插入或更新赛事数据
func SaveTournament(t models.Tournament) error {
	query := `INSERT INTO tournaments (id, name, year, status)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			year = excluded.year,
			status = excluded.status`
	_, err := DB.Exec(query, t.ID, t.Name, t.Year, t.Status)
	return err
}

// SaveMatch 插入或更新比赛数据
func SaveMatch(m models.Match) error {
	query := `INSERT INTO matches (id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			home_team = excluded.home_team,
			away_team = excluded.away_team,
			match_group = excluded.match_group,
			scheduled_at = excluded.scheduled_at,
			status = excluded.status,
			home_score = excluded.home_score,
			away_score = excluded.away_score,
			venue = excluded.venue`
	_, err := DB.Exec(query, m.ID, m.TournamentID, m.HomeTeam, m.AwayTeam, m.Group, m.ScheduledAt, m.Status, m.HomeScore, m.AwayScore, m.Venue)
	return err
}

// GetMatchesByTournament 按赛事ID获取所有比赛
func GetMatchesByTournament(tournamentID string) ([]models.Match, error) {
	query := `SELECT id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue
		FROM matches WHERE tournament_id = ? ORDER BY scheduled_at ASC`
	rows, err := DB.Query(query, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var scheduledStr string
		err := rows.Scan(&m.ID, &m.TournamentID, &m.HomeTeam, &m.AwayTeam, &m.Group, &scheduledStr, &m.Status, &m.HomeScore, &m.AwayScore, &m.Venue)
		if err != nil {
			return nil, err
		}
		m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05 -0700 -0700", scheduledStr)
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05-07:00", scheduledStr)
		}
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse(time.RFC3339, scheduledStr)
		}
		matches = append(matches, m)
	}
	return matches, nil
}

// GetMatch 获取单场比赛
func GetMatch(matchID string) (models.Match, error) {
	query := `SELECT id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue
		FROM matches WHERE id = ?`
	row := DB.QueryRow(query, matchID)
	var m models.Match
	var scheduledStr string
	err := row.Scan(&m.ID, &m.TournamentID, &m.HomeTeam, &m.AwayTeam, &m.Group, &scheduledStr, &m.Status, &m.HomeScore, &m.AwayScore, &m.Venue)
	if err != nil {
		return models.Match{}, err
	}
	m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05 -0700 -0700", scheduledStr)
	if m.ScheduledAt.IsZero() {
		m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05-07:00", scheduledStr)
	}
	if m.ScheduledAt.IsZero() {
		m.ScheduledAt, _ = time.Parse(time.RFC3339, scheduledStr)
	}
	return m, nil
}

// GetMatchesByTeam 获取指定球队在指定赛事中的所有比赛
func GetMatchesByTeam(tournamentID, teamName string) ([]models.Match, error) {
	query := `SELECT id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue
		FROM matches WHERE tournament_id = ? AND (home_team = ? OR away_team = ?) ORDER BY scheduled_at ASC`
	rows, err := DB.Query(query, tournamentID, teamName, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var scheduledStr string
		err := rows.Scan(&m.ID, &m.TournamentID, &m.HomeTeam, &m.AwayTeam, &m.Group, &scheduledStr, &m.Status, &m.HomeScore, &m.AwayScore, &m.Venue)
		if err != nil {
			return nil, err
		}
		m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05 -0700 -0700", scheduledStr)
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05-07:00", scheduledStr)
		}
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse(time.RFC3339, scheduledStr)
		}
		matches = append(matches, m)
	}
	return matches, nil
}

// GetMatchesByGroup 获取指定小组的所有比赛
func GetMatchesByGroup(tournamentID, group string) ([]models.Match, error) {
	query := `SELECT id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue
		FROM matches WHERE tournament_id = ? AND match_group = ? ORDER BY scheduled_at ASC`
	rows, err := DB.Query(query, tournamentID, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var scheduledStr string
		err := rows.Scan(&m.ID, &m.TournamentID, &m.HomeTeam, &m.AwayTeam, &m.Group, &scheduledStr, &m.Status, &m.HomeScore, &m.AwayScore, &m.Venue)
		if err != nil {
			return nil, err
		}
		m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05 -0700 -0700", scheduledStr)
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05-07:00", scheduledStr)
		}
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse(time.RFC3339, scheduledStr)
		}
		matches = append(matches, m)
	}
	return matches, nil
}

// UpdateMatchScore 仅更新比赛的比分和状态（赛程保护：不修改时间/主客队/场馆）
func UpdateMatchScore(matchID string, homeScore, awayScore int, status string) error {
	_, err := DB.Exec(`UPDATE matches SET home_score = ?, away_score = ?, status = ? WHERE id = ?`,
		homeScore, awayScore, status, matchID)
	return err
}

// UpdateMatchTeams 更新比赛的主客队队名
func UpdateMatchTeams(matchID string, homeTeam, awayTeam string) error {
	_, err := DB.Exec(`UPDATE matches SET home_team = ?, away_team = ? WHERE id = ?`,
		homeTeam, awayTeam, matchID)
	return err
}

