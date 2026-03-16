package appcontext

import (
	"context"
)

type contextKey string

const (
	userEmailKey  contextKey = "userEmail"
	isLoggedInKey contextKey = "isLoggedIn"
	hasPasskeyKey contextKey = "hasPasskey"
	userRoleKey   contextKey = "userRole"
	userIDKey     contextKey = "userID"
)

func WithUser(ctx context.Context, email string, loggedIn bool, hasPasskey bool, role string, id int64) context.Context {
	ctx = context.WithValue(ctx, userEmailKey, email)
	ctx = context.WithValue(ctx, isLoggedInKey, loggedIn)
	ctx = context.WithValue(ctx, hasPasskeyKey, hasPasskey)
	ctx = context.WithValue(ctx, userRoleKey, role)
	ctx = context.WithValue(ctx, userIDKey, id)
	return ctx
}

func GetUser(ctx context.Context) (string, bool, bool) {
	email, _ := ctx.Value(userEmailKey).(string)
	loggedIn, _ := ctx.Value(isLoggedInKey).(bool)
	hasPasskey, _ := ctx.Value(hasPasskeyKey).(bool)
	return email, loggedIn, hasPasskey
}

func GetUserRole(ctx context.Context) string {
	role, _ := ctx.Value(userRoleKey).(string)
	return role
}

func GetUserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}
