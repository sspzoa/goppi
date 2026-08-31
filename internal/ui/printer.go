package ui

type Printer struct {
	s *Stream
}

func NewPrinter() *Printer {
	return &Printer{s: NewStream()}
}

func (p *Printer) Delta(reasoning, content string) {
	if p.s == nil {
		p.s = NewStream()
	}
	p.s.Write(reasoning, content)
}

func (p *Printer) TurnEnd() {
	if p.s != nil {
		p.s.Close()
	}
	p.s = NewStream()
}

func (p *Printer) Usage(in, out, reason int) {
	Usage(in, out, reason)
}

func (p *Printer) ToolStart(name, detail string) {
	ToolCall(name, detail)
}

func (p *Printer) ToolDone(summary string, err error) {
	if err != nil {
		ToolFail(err)
		return
	}
	ToolOK(summary)
}

func (p *Printer) Compacted() {
	Info("세션을 자동 압축했습니다.")
}
