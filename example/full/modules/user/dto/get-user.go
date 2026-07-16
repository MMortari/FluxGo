package dto

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/entities"
)

type GetUserReq struct {
	IdUser string `json:"id_user" jsonschema:"required,description=Identificador do usuário"`
}
type GetUserRes struct {
	User        entities.User           `json:"user"`
	Permissions []fluxgo.PermissionRule `json:"permissions"`
}
