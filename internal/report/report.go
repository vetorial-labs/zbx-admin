package report

import "time"

// HostResult é a disponibilidade de um host no período.
type HostResult struct {
	ID          string
	Technical   string
	Visible     string
	HasICMP     bool
	Source      string
	UpSeconds   int64
	DownSeconds int64
	Outages     []Outage
}

// Accounted é UP + DOWN. Para host com ICMP, coincide com a duração do período.
func (h HostResult) Accounted() int64 {
	return h.UpSeconds + h.DownSeconds
}

// Report é o documento exportado.
type Report struct {
	Target    string
	Kind      string
	Period    Period
	Generated time.Time
	ServerURL string
	Version   string
	Capped    bool
	Hosts     []HostResult
}

// Totals soma apenas os hosts que têm item icmpping, em segundos.
func (r Report) Totals() (up, down, accounted int64, hosts int) {
	for _, h := range r.Hosts {
		if !h.HasICMP {
			continue
		}
		up += h.UpSeconds
		down += h.DownSeconds
		accounted += h.Accounted()
		hosts++
	}
	return up, down, accounted, hosts
}

// DisplayTotals soma as horas já arredondadas, em centésimos de hora.
// UP + DOWN de cada host fecha com o período, e o total fecha com a soma das linhas.
func (r Report) DisplayTotals() (up, down, accounted int64, hosts int) {
	period := r.Period.Seconds()
	for _, h := range r.Hosts {
		if !h.HasICMP {
			continue
		}
		u, d, a := SplitHours(period, h.DownSeconds)
		up += u
		down += d
		accounted += a
		hosts++
	}
	return up, down, accounted, hosts
}

// WithoutICMP lista os hosts que ficaram fora da soma.
func (r Report) WithoutICMP() []HostResult {
	var out []HostResult
	for _, h := range r.Hosts {
		if !h.HasICMP {
			out = append(out, h)
		}
	}
	return out
}

// NewHostResult monta a linha de um host monitorado por ICMP.
func NewHostResult(id, technical, visible, source string, periodSeconds, down int64, outages []Outage) HostResult {
	if down < 0 {
		down = 0
	}
	if down > periodSeconds {
		down = periodSeconds
	}
	return HostResult{
		ID:          id,
		Technical:   technical,
		Visible:     visible,
		HasICMP:     true,
		Source:      source,
		UpSeconds:   periodSeconds - down,
		DownSeconds: down,
		Outages:     outages,
	}
}
