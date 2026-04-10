package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/cndingbo2030/dingovault/internal/ai"
	"github.com/cndingbo2030/dingovault/internal/config"
	"github.com/cndingbo2030/dingovault/internal/locale"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartAIInlineStream begins a streaming LLM refactor for one block. Chunks are emitted as Wails events:
//   - "ai-inline-chunk"  payload: { opID, chunk }
//   - "ai-inline-done"    payload: { opID }
//   - "ai-inline-error"   payload: { opID, message }
func (a *App) StartAIInlineStream(opID, blockID, instruction string) error {
	opID = strings.TrimSpace(opID)
	blockID = strings.TrimSpace(blockID)
	inst := strings.TrimSpace(instruction)
	if opID == "" || blockID == "" {
		return fmt.Errorf("missing op or block id")
	}
	if inst == "" {
		return fmt.Errorf("%s", a.t(locale.ErrAIEmptyInstruction))
	}
	if a.ctx == nil {
		return fmt.Errorf("app not ready")
	}
	if a.store == nil {
		return fmt.Errorf("%s", a.t(locale.ErrStoreNotInit))
	}
	go a.runAIInlineStream(opID, blockID, inst)
	return nil
}

func (a *App) runAIInlineStream(opID, blockID, instruction string) {
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	emit := func(name string, payload map[string]any) {
		if a.EventEmitter != nil {
			a.EventEmitter(name, payload)
			return
		}
		if a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, name, payload)
	}

	success := false
	defer func() {
		if success {
			emit("ai-inline-done", map[string]any{"opID": opID})
		}
	}()

	b, err := a.store.GetBlockByID(ctx, blockID)
	if err != nil {
		emit("ai-inline-error", map[string]any{"opID": opID, "message": err.Error()})
		return
	}

	c, err := config.Load()
	if err != nil {
		c = config.Default()
	}
	c.AI = config.NormalizeAISettings(c.AI)
	p, err := ai.NewProvider(c.AI)
	if err != nil {
		emit("ai-inline-error", map[string]any{"opID": opID, "message": err.Error()})
		return
	}
	msgs := ai.BuildInlineRefactorMessages(b.Content, instruction)
	err = p.StreamComplete(ctx, msgs, func(chunk string) error {
		if chunk == "" {
			return nil
		}
		emit("ai-inline-chunk", map[string]any{"opID": opID, "chunk": chunk})
		return nil
	})
	if err != nil {
		emit("ai-inline-error", map[string]any{"opID": opID, "message": err.Error()})
		return
	}
	success = true
}
