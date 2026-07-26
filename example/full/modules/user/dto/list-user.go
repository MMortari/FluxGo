package dto

import (
	"github.com/MMortari/FluxGo/example/full/shared/entities"
)

type PaginatedQuery struct {
	Page     int `query:"page" validate:"min=1"`
	PageSize int `query:"page_size" validate:"min=1,max=100"`
}

type ListUserReq struct {
	PaginatedQuery
	IdUser *string `query:"id_user" jsonschema:"title=Identificador do usuário"`
	Name   *string `query:"name" jsonschema:"title=Nome do usuário,default=João da Silva,maxLength=150"`
}
type ListUserRes struct {
	Data []entities.User `json:"data"`
}
