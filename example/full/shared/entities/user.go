package entities

type User struct {
	ID   string `json:"id" jsonschema:"title=Identificador do usuário"`
	Name string `json:"name" jsonschema:"title=Nome"`
}

func (u User) TableName() string {
	return "user"
}

func (u User) PrimaryKey() string {
	return "id"
}
