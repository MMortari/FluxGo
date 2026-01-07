package dto

import "github.com/MMortari/FluxGo/example/full/shared/entities"

type GetUserReq struct {
	IdUser string `json:"id_user" jsonschema:"required,description=Identificador do usuário"`
}
type GetUserRes struct {
	User entities.User `json:"user"`
}
