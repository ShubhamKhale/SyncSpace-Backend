package database

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// EmailTemplateRepo fetches email templates from the database and caches them
// in-process via sync.Map. Each template is loaded at most once per process
// lifetime — cache is never evicted. To pick up a template change, restart.
type EmailTemplateRepo struct {
	db    *pgxpool.Pool
	cache sync.Map // key: template name (string) → *model.EmailTemplate
}

// NewEmailTemplateRepo creates an EmailTemplateRepo backed by the given pool.
func NewEmailTemplateRepo(db *pgxpool.Pool) *EmailTemplateRepo {
	return &EmailTemplateRepo{db: db}
}

// Get returns the template with the given name. On first call the template is
// fetched from the DB and stored in the sync.Map; subsequent calls return the
// cached value without hitting the database.
func (r *EmailTemplateRepo) Get(ctx context.Context, name string) (*model.EmailTemplate, error) {
	if v, ok := r.cache.Load(name); ok {
		return v.(*model.EmailTemplate), nil
	}

	tpl := &model.EmailTemplate{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, subject, html_body, created_at, updated_at
		 FROM public.email_templates WHERE name = $1`,
		name,
	).Scan(&tpl.ID, &tpl.Name, &tpl.Subject, &tpl.HTMLBody, &tpl.CreatedAt, &tpl.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("email template not found: " + name)
		}
		return nil, errs.Internal("failed to query email template")
	}

	r.cache.Store(name, tpl)
	return tpl, nil
}
