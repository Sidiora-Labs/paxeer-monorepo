package analytics

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func SetTokenInputsBeforeCommit(p *Projector, hook func(context.Context, pgx.Tx) error) {
	p.tokenInputsBeforeCommit = hook
}
