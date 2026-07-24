package dto

import (
	"github.com/MMortari/FluxGo/example/full/shared/entities"
)

type ListUserReq struct {
	IdUser *string `query:"id_user" jsonschema:"title=Identificador do usuário"`
	Name   *string `query:"name" jsonschema:"title=Nome do usuário,default=João da Silva,maxLength=150"`
}
type ListUserRes struct {
	Data []entities.User `json:"data"`
}
