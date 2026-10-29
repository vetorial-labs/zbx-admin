package resolve

import "testing"

func TestPickExactWinsOverPartials(t *testing.T) {
	list := []Candidate{
		{ID: "1", Primary: "ICMP-2", Secondary: "icmp-2"},
		{ID: "2", Primary: "ICMP", Secondary: "icmp"},
		{ID: "3", Primary: "outro ICMP host", Secondary: "outro"},
	}
	got, err := Pick("host", "icmp", list)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "2" {
		t.Fatalf("id %s", got.ID)
	}
}

func TestPickSinglePartial(t *testing.T) {
	got, err := Pick("host", "blanken", []Candidate{
		{ID: "10775", Primary: "BLANKEN LTDA", Secondary: "BLANKEN LTDA"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "10775" {
		t.Fatalf("id %s", got.ID)
	}
}

func TestPickAmbiguous(t *testing.T) {
	_, err := Pick("grupo", "cli", []Candidate{
		{ID: "32", Primary: "CLIENTES"},
		{ID: "50", Primary: "Radio Clientes"},
	})
	var amb *AmbiguousError
	if !asAmbiguous(err, &amb) {
		t.Fatalf("erro %v", err)
	}
	if len(amb.Options) != 2 {
		t.Fatalf("opções %d", len(amb.Options))
	}
}

func TestPickNotFound(t *testing.T) {
	_, err := Pick("host", "nao-existe", nil)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if _, ok := err.(*AmbiguousError); ok {
		t.Fatal("ausência não é ambiguidade")
	}
}

func asAmbiguous(err error, dest **AmbiguousError) bool {
	amb, ok := err.(*AmbiguousError)
	if ok {
		*dest = amb
	}
	return ok
}
