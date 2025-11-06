package auth

import (
	"fmt"

	"go.uber.org/zap"
)

type Authenticator struct {
	allowedUserIDs []int64
	logger         *zap.Logger
}

func NewAuthenticator(allowedUserIDs []int64, logger *zap.Logger) *Authenticator {
	return &Authenticator{
		allowedUserIDs: allowedUserIDs,
		logger:         logger,
	}
}

func (a *Authenticator) IsUserAllowed(userID int64) bool {
	for _, allowedID := range a.allowedUserIDs {
		if userID == allowedID {
			a.logger.Info("User access granted",
				zap.Int64("user_id", userID))
			return true
		}
	}

	a.logger.Warn("Unauthorized access attempt",
		zap.Int64("user_id", userID))
	return false
}

func (a *Authenticator) GetAllowedUsersCount() int {
	return len(a.allowedUserIDs)
}

func (a *Authenticator) GetUserInfo(userID int64) string {
	for _, allowedID := range a.allowedUserIDs {
		if userID == allowedID {
			return fmt.Sprintf("User %d (authorized)", userID)
		}
	}
	return fmt.Sprintf("User %d (unauthorized)", userID)
}