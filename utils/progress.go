package utils

import (
	"context"
	"fmt"
	"github.com/cheggaaa/pb/v3"
)

var (
	GuiProgressCallback func(current, total int)
	GuiLiveCallback     func(data CloudflareIPData)
	GuiSpeedCallback    func() // Just a signal to refresh Fyne table
	CancelCtx           context.Context
	CancelFunc          context.CancelFunc
)

type Bar struct {
	pb    *pb.ProgressBar
	total int
	count int
}

func NewBar(count int, MyStrStart, MyStrEnd string) *Bar {
	tmpl := fmt.Sprintf(`{{counters . }} {{ bar . "[" "-" (cycle . "↖" "↗" "↘" "↙" ) "_" "]"}} %s {{string . "MyStr" | green}} %s `, MyStrStart, MyStrEnd)
	bar := pb.ProgressBarTemplate(tmpl).Start(count)
	return &Bar{pb: bar, total: count, count: 0}
}

func (b *Bar) Grow(num int, MyStrVal string) {
	b.count += num
	b.pb.Set("MyStr", MyStrVal).Add(num)
	if GuiProgressCallback != nil {
		GuiProgressCallback(b.count, b.total)
	}
}

func (b *Bar) Done() {
	b.pb.Finish()
	if GuiProgressCallback != nil {
		GuiProgressCallback(b.total, b.total)
	}
}
