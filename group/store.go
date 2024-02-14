package group

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type Store struct {
	db *sqlx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{
		db: db,
	}
}

type Group struct {
	Id               string    `db:"id"`
	CreatedAt        time.Time `db:"created_at"`
	IsDeleted        bool      `db:"is_deleted"`
	Name             string    `db:"name"`
	InviteId         string    `db:"invite_id"`
	TotalMemberCount int       `db:"total_member_count"`
}

type GroupMember struct {
	GroupId   string    `db:"group_id"`
	UserId    string    `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`

	UserFullName string `db:"user_full_name"`
}

type GroupDetailed struct {
	Group
	Members []GroupMember
}

type CreateParams struct {
	Id       string
	Name     string
	InviteId string
}

func (s *Store) Create(p CreateParams) error {
	stmt := `
        INSERT INTO user_group (id, created_at, name, invite_id)
        VALUES (?, ?, ?, ?)
    `
	args := []any{
		p.Id,
		time.Now().UTC(),
		p.Name,
		p.InviteId,
	}

	_, err := s.db.Exec(stmt, args...)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) Get(id string) (Group, error) {
	stmt := `
        SELECT id, name, invite_id FROM user_group
        WHERE id = ? AND is_deleted = FALSE
    `
	args := []any{id}

	var g Group
	err := s.db.Get(&g, stmt, args...)
	return g, err
}

func (s *Store) GetMembers(id string) ([]GroupMember, error) {
	stmt := `
        SELECT ugm.group_id, ugm.user_id, u.full_name AS user_full_name FROM user_group_member ugm
        INNER JOIN user u ON u.id = ugm.user_id
        WHERE group_id = ?
    `
	args := []any{id}

	var m []GroupMember
	err := s.db.Select(&m, stmt, args...)
	return m, err
}

func (s *Store) List() ([]Group, error) {
	stmt := `
        SELECT 
            ug.id, ug.name
            , COALESCE(ugc.total_member_count, 0) AS total_member_count
        FROM user_group AS ug
        LEFT JOIN (
            SELECT group_id, COUNT(*) AS total_member_count FROM user_group_member
            GROUP BY group_id
        ) AS ugc ON ug.id = ugc.group_id
        WHERE is_deleted = FALSE
        ORDER BY created_at ASC
    `

	var g []Group
	err := s.db.Select(&g, stmt)
	return g, err
}

func (s *Store) GetByInviteId(inviteId string) (Group, error) {
	stmt := `
        SELECT id, name FROM user_group
        WHERE invite_id = ? AND is_deleted = FALSE
    `
	args := []any{inviteId}

	var g Group
	err := s.db.Get(&g, stmt, args...)
	return g, err
}

func (s *Store) HasMember(groupId, userId string) (bool, error) {
	stmt := `
        SELECT 1 FROM user_group_member
        WHERE group_id = ? AND user_id = ?
    `
	args := []any{groupId, userId}

	var i int
	err := s.db.Get(&i, stmt, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Store) AddMember(groupId string, userId string) error {
	stmt := `
        INSERT INTO user_group_member (group_id, user_id, created_at)
        VALUES (?, ?, ?)
    `
	args := []any{
		groupId,
		userId,
		time.Now().UTC(),
	}

	_, err := s.db.Exec(stmt, args...)
	return err
}

type UpdateParams struct {
	Id   string
	Name string
}

func (s *Store) Update(p UpdateParams) error {
	stmt := `
        UPDATE user_group
        SET name = ?
        WHERE id = ?
    `
	args := []any{p.Name, p.Id}

	_, err := s.db.Exec(stmt, args...)
	return err
}

func (s *Store) Delete(id string) error {
	stmt := `
        UPDATE user_group
        SET is_deleted = TRUE
        WHERE id = ?
    `
	args := []any{id}

	_, err := s.db.Exec(stmt, args...)
	return err
}

func (s *Store) RemoveMember(groupId string, userId string) error {
    stmt := `
        DELETE FROM user_group_member
        WHERE group_id = ? AND user_id = ?
    `
    args := []any{groupId, userId}

    _, err := s.db.Exec(stmt, args...)
    return err
}