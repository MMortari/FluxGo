package dto

import "github.com/MMortari/FluxGo/example/full/shared/entities"

type GetUserRes struct {
	User entities.User `json:"user"`
}
