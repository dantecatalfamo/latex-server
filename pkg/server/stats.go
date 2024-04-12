package server

import (
	_ "embed"
	"fmt"
)

type UserStats struct {
	UserID uint64 `json:"user_id"`
	UserName string `json:"user_name"`
	ProjectID uint64 `json:"project_id"`
	ProjectName string `json:"project_name"`
	TotalBuilds uint64 `json:"total_builds"`
	TotalBuildTime float64 `json:"total_buil_time"`
	TotalFiles uint64 `json:"total_files"`
	TotalFileSize uint64 `json:"total_file_size"`
}


//go:embed stats.sql
var stats_query string

func GetGlobalStats(config Config) ([]UserStats, error) {
	rows, err := config.database.conn.Query(stats_query)
	if err != nil {
		return nil, fmt.Errorf("GetGlobalStats query: %w", err)
	}
	defer rows.Close()

	var userStats []UserStats

	for rows.Next() {
		var userStat UserStats
		if err := rows.Scan(
			&userStat.UserID,
			&userStat.UserName,
			&userStat.ProjectID,
			&userStat.ProjectName,
			&userStat.TotalBuilds,
			&userStat.TotalBuildTime,
			&userStat.TotalFiles,
			&userStat.TotalFileSize,
		); err != nil {
			return nil, fmt.Errorf("GetGlobalStats scan: %w", err)
		}

		userStats = append(userStats, userStat)
	}

	return userStats, nil
}
