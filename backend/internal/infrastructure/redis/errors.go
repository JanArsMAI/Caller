package redisrepo

import "errors"

var (
	ErrClientNotFound    = errors.New("client not found")
	ErrRoomNotFound      = errors.New("room not found")
	ErrClientNotInRoom   = errors.New("client not in room")
	ErrRoomAlreadyExists = errors.New("room already exists")
	ErrInvalidData       = errors.New("invalid data format")
	ErrRedisNotConnected = errors.New("redis not connected")
)
