package repositories

import (
	"context"
	"database/sql"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/entities"
)

type UserRepository struct {
	fluxgo.Repository[entities.User]
}

func UserRepositoryStart(db *fluxgo.Database) *UserRepository {
	return &UserRepository{*fluxgo.NewRepository[entities.User](db)}
}

func (r *UserRepository) GetUser(ctx context.Context) (*entities.User, error) {
	ctx, span := r.StartSpan(ctx)
	defer span.End()

	var user entities.User

	err := r.DB.Get(&user, "SELECT '299f3dcd-42f3-46c1-89d5-603c78a78f50' as id, 'John' AS name")
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	return &user, nil
}
