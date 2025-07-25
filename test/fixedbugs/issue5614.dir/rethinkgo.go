package rethinkgo

type Session struct {
}

func (s *Session) Run(query Exp) *int { return nil }

type List []any

type Exp struct {
	args []any
}

func (e Exp) UseOutdated(useOutdated bool) Exp {
	return Exp{args: List{e, useOutdated}}
}
