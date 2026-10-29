// Package resolve escolhe um único host ou grupo a partir da busca.
package resolve

import (
	"fmt"
	"strings"
)

// Candidate é um host ou grupo devolvido pela busca.
type Candidate struct {
	ID        string
	Primary   string
	Secondary string
}

// AmbiguousError indica que a busca precisa ser refinada. A CLI sai com código 2.
type AmbiguousError struct {
	Kind    string
	Term    string
	Options []Candidate
}

func (e *AmbiguousError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "mais de um %s para %q:\n", e.Kind, e.Term)
	for _, c := range e.Options {
		if c.Secondary != "" {
			fmt.Fprintf(&b, "  %s\t%s\t%s\n", c.ID, c.Secondary, c.Primary)
		} else {
			fmt.Fprintf(&b, "  %s\t%s\n", c.ID, c.Primary)
		}
	}
	b.WriteString("Refine a busca ou informe o ID.")
	return b.String()
}

// Pick devolve o único candidato.
// Um nome exato (sem diferenciar maiúsculas) vence vários resultados parciais.
func Pick(kind, term string, list []Candidate) (Candidate, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return Candidate{}, fmt.Errorf("informe o %s para a busca", kind)
	}
	if len(list) == 0 {
		return Candidate{}, fmt.Errorf("nenhum %s encontrado para %q", kind, term)
	}
	var exact []Candidate
	for _, c := range list {
		if same(c.Primary, term) || (c.Secondary != "" && same(c.Secondary, term)) || c.ID == term {
			exact = append(exact, c)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return Candidate{}, &AmbiguousError{Kind: kind, Term: term, Options: exact}
	}
	if len(list) == 1 {
		return list[0], nil
	}
	return Candidate{}, &AmbiguousError{Kind: kind, Term: term, Options: list}
}

func same(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
