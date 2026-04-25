package app

import (
	"testing"

	"example.com/haohao/backend/internal/service"
)

func TestRateLimitRequesterIDUsesSupportActor(t *testing.T) {
	actor := service.User{ID: 100}
	current := service.CurrentSession{
		User:      service.User{ID: 200},
		ActorUser: &actor,
	}

	if got := rateLimitRequesterID(current); got != actor.ID {
		t.Fatalf("rateLimitRequesterID() = %d, want actor %d", got, actor.ID)
	}
}

func TestRateLimitRequesterIDFallsBackToCurrentUser(t *testing.T) {
	current := service.CurrentSession{
		User: service.User{ID: 200},
	}

	if got := rateLimitRequesterID(current); got != current.User.ID {
		t.Fatalf("rateLimitRequesterID() = %d, want user %d", got, current.User.ID)
	}
}
