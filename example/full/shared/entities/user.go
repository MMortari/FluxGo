package entities

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (u User) TableName() string {
	return "user"
}

func (u User) PrimaryKey() string {
	return "id"
}
