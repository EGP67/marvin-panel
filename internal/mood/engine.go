package mood

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"hog.local/marvin-panel/internal/model"
)

// Selection and pacing parameters (D-058, docs/MARVIN.md).
const (
	Rotate          = 120 * time.Second
	DoomReassert    = 60 * time.Second
	LineCooldown    = 10 * time.Minute
	FamilyCooldown  = 3 * time.Minute
	GPUHold         = 30 * time.Second
	GPURefresh      = 5 * time.Minute
	defaultStateDir = "/var/lib/heartofgold"
)

// Options configures an Engine.
type Options struct {
	StateDir string       // PANIC COUNT directory; default /var/lib/heartofgold
	Log      *slog.Logger // default slog.Default()
}

// Engine fills mood, phrase.lines, gpu_line and panic_count. It is not safe for
// concurrent use: one goroutine calls Apply.
type Engine struct {
	log   *slog.Logger
	mood  moodState
	panic *panicStore
	lines []line
	errs  map[string]string

	cur        *line
	selectedAt time.Time
	selMood    string
	lastShown  map[string]time.Time
	famShown   map[string]time.Time

	gpuSet       bool
	gpuKind      model.GPULineKind
	gpuID        string
	gpuText      string
	gpuAt        time.Time
	gpuPending   model.GPULineKind
	gpuPendSince time.Time
	gpuShown     map[string]time.Time
}

// New loads the PANIC COUNT (creating 0 on first run); a load error is logged once and
// the count renders null.
func New(o Options) *Engine {
	e := &Engine{log: o.Log, lines: table, errs: map[string]string{},
		lastShown: map[string]time.Time{}, famShown: map[string]time.Time{}, gpuShown: map[string]time.Time{}}
	if e.log == nil {
		e.log = slog.Default()
	}
	dir := o.StateDir
	if dir == "" {
		dir = defaultStateDir
	}
	p, err := loadPanic(dir)
	e.panic = p
	e.note("panic_count", err)
	return e
}

func (e *Engine) note(source string, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if e.errs[source] == msg {
		return
	}
	e.errs[source] = msg
	if err != nil {
		e.log.Warn("mood engine", "source", source, "err", msg)
	}
}

// Mood is the current mood ("" before the first complete sample).
func (e *Engine) Mood() string { return e.mood.current }

// Apply sets the engine-chosen fields of s at time now (local time decides the night line).
func (e *Engine) Apply(s *model.Snapshot, now time.Time) {
	s.GPULine = e.gpuLine(s, now)
	if e.panic.count != nil {
		n := *e.panic.count
		s.PanicCount = &n
	}
	if !e.mood.adopted && s.CPU.TotalPct == nil {
		s.Mood = Content
		s.Phrase.Lines, _ = wrap(fallbackLine.tpl)
		return
	}
	if e.mood.step(s, now) && e.mood.current == Doomed {
		e.note("panic_count write", e.panic.increment())
		if e.panic.count != nil {
			n := *e.panic.count
			s.PanicCount = &n
		}
	}
	s.Mood = e.mood.current
	s.Phrase.Lines = e.phrase(s, now)
}

// eligible renders l for v; ok is false when the line may not speak now.
func (e *Engine) eligible(l *line, v view) ([]string, bool) {
	if !l.allows(v.mood) {
		return nil, false
	}
	text, ok := l.text(v)
	if !ok {
		return nil, false
	}
	if l.gpuHeat && v.gpuHot && v.mood != Doomed {
		if h, ok := hottest(v.s); ok && strings.HasPrefix(h.name, "GPU") {
			return nil, false // topic rule (D-059): the GPU line already says it
		}
	}
	return wrap(text)
}

func (e *Engine) phrase(s *model.Snapshot, now time.Time) []string {
	v := view{s: s, mood: e.mood.current, dwell: e.mood.dwell(now), now: now, gpuHot: e.gpuKind == model.GPUHot}
	interval := Rotate
	if v.mood == Doomed {
		interval = DoomReassert
	}
	if e.cur != nil && e.selMood == v.mood && now.Sub(e.selectedAt) < interval {
		if out, ok := e.eligible(e.cur, v); ok {
			e.shown(e.cur, now)
			return out
		}
		if e.cur.id == fallbackLine.id {
			// The fallback stays until its interval; reselect if something became true.
			if pick := e.pick(v, now); pick == nil {
				out, _ := e.eligible(e.cur, v)
				e.shown(e.cur, now)
				return out
			}
		}
	}
	l := e.pick(v, now)
	if l == nil {
		l = &fallbackLine
	}
	e.cur, e.selectedAt, e.selMood = l, now, v.mood
	text, _ := l.text(v)
	out, _ := wrap(text)
	e.shown(l, now)
	return out
}

// pick chooses the least recently shown eligible line (table order breaks ties). In
// doomed, trigger lines re-assert ignoring cooldowns; otherwise the exact-line and
// family cooldowns apply, except to the line already on screen (continuation is not a
// repeat).
func (e *Engine) pick(v view, now time.Time) *line {
	var best *line
	bestAt := time.Time{}
	consider := func(l *line, cooled bool) {
		if _, ok := e.eligible(l, v); !ok {
			return
		}
		if cooled && l != e.cur {
			if t, ok := e.lastShown[l.id]; ok && now.Sub(t) < LineCooldown {
				return
			}
			if t, ok := e.famShown[l.family]; ok && now.Sub(t) < FamilyCooldown {
				return
			}
		}
		at := e.lastShown[l.id]
		if best == nil || at.Before(bestAt) {
			best, bestAt = l, at
		}
	}
	if v.mood == Doomed {
		for i := range e.lines {
			if e.lines[i].allows(Doomed) {
				consider(&e.lines[i], false)
			}
		}
		if best != nil {
			return best
		}
	}
	for i := range e.lines {
		consider(&e.lines[i], true)
	}
	return best
}

func (e *Engine) shown(l *line, now time.Time) {
	e.lastShown[l.id] = now
	e.famShown[l.family] = now
}

// gpuLine paces the GRAPHICS line (D-058, D-059): a new state shows after GPUHold; within
// a state the pool rotates every GPURefresh, least recently shown first, with the
// 10-minute repeat rule per line id; the line on screen continues if nothing else
// qualifies. The first call adopts at once. Text is rendered at selection.
func (e *Engine) gpuLine(s *model.Snapshot, now time.Time) string {
	states := make([]model.GPUState, 0, len(s.GPUs))
	for _, g := range s.GPUs {
		states = append(states, model.GPUState{TempC: g.TempC, UtilPct: g.UtilPct, MemUsedMiB: g.MemUsedMiB})
	}
	kind := model.GPULineKindOf(states)
	switch {
	case !e.gpuSet:
		e.gpuSet = true
		e.selectGPU(kind, states, now)
	case kind != e.gpuKind:
		if e.gpuPending != kind {
			e.gpuPending, e.gpuPendSince = kind, now
		}
		if now.Sub(e.gpuPendSince) >= GPUHold {
			e.gpuPending = ""
			e.selectGPU(kind, states, now)
		}
	default:
		e.gpuPending = ""
		if now.Sub(e.gpuAt) >= GPURefresh {
			e.selectGPU(kind, states, now)
		}
	}
	e.gpuShown[e.gpuID] = now
	return e.gpuText
}

func (e *Engine) selectGPU(kind model.GPULineKind, states []model.GPUState, now time.Time) {
	pool := model.GPULinePools[kind]
	pick := func(cooled bool) (string, string, bool) {
		bestID, bestText, found := "", "", false
		var bestAt time.Time
		for i, tpl := range pool {
			id := fmt.Sprintf("%s-%d", kind, i+1)
			text, ok := model.RenderGPULine(tpl, states)
			if !ok {
				continue
			}
			at, shown := e.gpuShown[id]
			if cooled && shown && id != e.gpuID && now.Sub(at) < LineCooldown {
				continue
			}
			if !found || at.Before(bestAt) {
				bestID, bestText, bestAt, found = id, text, at, true
			}
		}
		return bestID, bestText, found
	}
	id, text, ok := pick(true)
	if !ok {
		id, text, ok = pick(false) // a new state must still say something true
	}
	if !ok {
		return
	}
	e.gpuKind, e.gpuID, e.gpuText, e.gpuAt = kind, id, text, now
}
