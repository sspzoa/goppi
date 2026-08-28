package agent

type Sink interface {
	Delta(reasoning, content string)
	TurnEnd()
	Usage(in, out, reason int)
	ToolStart(name, detail string)
	ToolDone(summary string, err error)
}

type nopSink struct{}

func (nopSink) Delta(string, string)     {}
func (nopSink) TurnEnd()                 {}
func (nopSink) Usage(int, int, int)      {}
func (nopSink) ToolStart(string, string) {}
func (nopSink) ToolDone(string, error)   {}
