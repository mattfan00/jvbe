package user

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/mattfan00/jvbe/db"
	"github.com/mattfan00/jvbe/logger"
)

type service struct {
	db  *db.DB
	log logger.Logger
}

func NewService(db *db.DB) *service {
	return &service{
		db:  db,
		log: logger.NewNoopLogger(),
	}
}

func (s *service) SetLogger(l logger.Logger) {
	s.log = l
}

func (s *service) Get(id string) (User, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	u, err := get(tx, id)
	return u, err
}

func (s *service) HandleFromExternal(externalUser ExternalUser) (User, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	user, err := getByExternal(tx, externalUser.Id)
	// if cant retrieve user, then need to create
	if errors.Is(err, ErrNoUser) {
		user, err = create(tx, CreateParams{
			ExternalId: externalUser.Id,
			FullName:   externalUser.FullName,
		})
		if err != nil {
			return User{}, err
		}
		s.log.Printf("created new user %s", user.Id)

		err = updateReview(tx, UpdateReviewParams{
			UserId: user.Id,
		})
		if err != nil {
			return User{}, err
		}

	} else if err != nil {
		return User{}, err
	}

	err = tx.Commit()
	if err != nil {
		return User{}, err
	}

	return user, nil
}

type CreateParams struct {
	ExternalId string
	FullName   string
	Picture    string
}

func (s *service) Create(p CreateParams) (User, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	u, err := create(tx, p)
	if err != nil {
		return User{}, err
	}

	err = tx.Commit()
	if err != nil {
		return User{}, err
	}

	return u, err
}

func (s *service) GetReview(userId string) (UserReview, error) {
	stmt := `
        SELECT user_id, comment FROM user_review
        WHERE user_id = ?
    `
	args := []any{userId}

	var userReview UserReview
	err := s.db.Get(&userReview, stmt, args...)
	return userReview, err
}

func (s *service) ListReviews() ([]UserReview, error) {
	stmt := `
        SELECT 
            ur.user_id, ur.created_at, ur.comment 
            , u.full_name AS user_full_name
        FROM user_review ur
        INNER JOIN user u ON ur.user_id = u.id
        WHERE is_approved = 0
        ORDER BY ur.created_at
    `

	var u []UserReview
	err := s.db.Select(&u, stmt)
	return u, err
}

type UpdateReviewParams struct {
	UserId    string
	CreatedAt time.Time
	Comment   string
}

func (s *service) UpdateReview(p UpdateReviewParams) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = updateReview(tx, UpdateReviewParams{
		UserId:  p.UserId,
		Comment: p.Comment,
	})

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *service) ApproveReview(userId string) error {
	if userId == "" {
		return errors.New("invalid user")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt := `
        UPDATE user_review
        SET is_approved = 1, reviewed_at = ?
        WHERE user_id = ?
    `
	args := []any{time.Now().UTC(), userId}

	_, err = tx.Exec(stmt, args...)
	if err != nil {
		return err
	}
	s.log.Printf("approved user review for %s", userId)

	stmt = `
        UPDATE user
        SET status = ?
        WHERE id = ?
    `
	args = []any{UserStatusActive, userId}

	_, err = tx.Exec(stmt, args...)
	if err != nil {
		return err
	}
	s.log.Printf("set user %s to active status", userId)

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func get(tx *sqlx.Tx, id string) (User, error) {
	stmt := `
        SELECT id, full_name, external_id, created_at, status FROM user
        WHERE id = ?
    `
	args := []any{id}

	var user User
	err := tx.Get(&user, stmt, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoUser
	} else if err != nil {
		return User{}, err
	}

	return user, nil
}

func getByExternal(tx *sqlx.Tx, externalId string) (User, error) {
	stmt := `
        SELECT 
            id, full_name, external_id, created_at, status
        FROM user
        WHERE external_id = ?
    `
	args := []any{externalId}

	var user User
	err := tx.Get(&user, stmt, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoUser
	} else if err != nil {
		return User{}, err
	}

	return user, nil
}

func create(tx *sqlx.Tx, p CreateParams) (User, error) {
	newId, err := gonanoid.New()
	if err != nil {
		return User{}, err
	}

	stmt := `
        INSERT INTO user (id, full_name, external_id, created_at, status)
        VALUES (?, ?, ?, ?, ?)
    `
	args := []any{
		newId,
		p.FullName,
		p.ExternalId,
		time.Now().UTC(),
		UserStatusInactive,
	}

	_, err = tx.Exec(stmt, args...)
	if err != nil {
		return User{}, err
	}

	newUser, err := get(tx, newId)
	if err != nil {
		return User{}, err
	}

	return newUser, nil
}

func updateReview(tx *sqlx.Tx, p UpdateReviewParams) error {
	if len(p.Comment) > 100 {
		return errors.New("comment too long")
	}

	stmt := `
        INSERT INTO user_review (user_id, created_at, comment)
        VALUES (?, ?, ?)
        ON CONFLICT (user_id) DO UPDATE SET
            comment = excluded.comment
    `
	args := []any{
		p.UserId,
		time.Now().UTC(),
		sql.NullString{
			String: p.Comment,
			Valid:  p.Comment != "",
		},
	}

	_, err := tx.Exec(stmt, args...)
	return err
}
