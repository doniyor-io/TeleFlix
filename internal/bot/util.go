package bot

import (
	"os"
	"strconv"
	"strings"
)

func IsAdmin(userID int64) bool {
	adminIDsEnv := os.Getenv("ADMIN_IDS")
	if adminIDsEnv == "" {
		return false
	}

	ids := strings.Split(adminIDsEnv, ",")
	for _, idStr := range ids {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			continue
		}
		if id == userID {
			return true
		}
	}
	return false
}
