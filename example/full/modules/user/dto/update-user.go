package dto

type UpdateUserReq struct {
	IdUser *string `params:"id_user" jsonschema:"title=Identificador do usuário"`
	Name   *string `json:"name" jsonschema:"title=Nome do usuário,default=João da Silva,maxLength=150"`
}
type UpdateUserRes struct {
	Success bool `json:"success"`
}
