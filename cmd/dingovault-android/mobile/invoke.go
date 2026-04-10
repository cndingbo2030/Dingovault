package mobile

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cndingbo2030/dingovault/internal/bridge"
)

// Invoke runs a Wails bridge method by name with a JSON array of arguments (same order as App.js).
// The returned string is always JSON: {"ok":true} or {"ok":true,"data":...} or {"ok":false,"error":"..."}.
func Invoke(method, argsJSON string) (string, error) {
	mu.Lock()
	a := app
	mu.Unlock()
	if a == nil {
		b, _ := json.Marshal(map[string]any{"ok": false, "error": "mobile: not initialized"})
		return string(b), nil
	}
	var args []json.RawMessage
	if argsJSON != "" && argsJSON != "null" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return wrapErr(err), nil
		}
	}
	data, err := dispatch(a, method, args)
	if err != nil {
		return wrapErr(err), nil
	}
	return wrapOK(data), nil
}

func wrapErr(err error) string {
	b, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	return string(b)
}

func wrapOK(data any) string {
	if data == nil {
		return `{"ok":true}`
	}
	b, err := json.Marshal(struct {
		OK   bool `json:"ok"`
		Data any  `json:"data"`
	}{OK: true, Data: data})
	if err != nil {
		return wrapErr(err)
	}
	return string(b)
}

func strArg(args []json.RawMessage, i int) (string, error) {
	if len(args) <= i {
		return "", fmt.Errorf("missing arg %d", i)
	}
	var s string
	if err := json.Unmarshal(args[i], &s); err != nil {
		return "", err
	}
	return s, nil
}

func intArg(args []json.RawMessage, i int) (int, error) {
	if len(args) <= i {
		return 0, fmt.Errorf("missing arg %d", i)
	}
	var n int
	if err := json.Unmarshal(args[i], &n); err == nil {
		return n, nil
	}
	var f float64
	if err := json.Unmarshal(args[i], &f); err != nil {
		return 0, err
	}
	return int(f), nil
}

func dispatch(a *bridge.App, method string, args []json.RawMessage) (any, error) {
	switch method {
	case "Startup":
		a.Startup(context.Background())
		return nil, nil
	case "GetAppVersion":
		return a.GetAppVersion(), nil
	case "NotesRoot":
		return a.NotesRoot(), nil
	case "GetLocale":
		return a.GetLocale()
	case "SetLocale":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.SetLocale(s)
	case "GetTheme":
		return a.GetTheme()
	case "SetTheme":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.SetTheme(s)
	case "SearchBlocks":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return a.SearchBlocks(s)
	case "ListVaultPages":
		return a.ListVaultPages()
	case "GetPage":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return a.GetPage(s)
	case "UpdateBlock":
		id, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		content, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return nil, a.UpdateBlock(id, content)
	case "InsertBlockAfter":
		id, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		t, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return nil, a.InsertBlockAfter(id, t)
	case "ReorderBlockBefore":
		a1, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		a2, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return nil, a.ReorderBlockBefore(a1, a2)
	case "GetWikiGraph":
		return a.GetWikiGraph()
	case "GetSemanticGraphEdges":
		return a.GetSemanticGraphEdges()
	case "IndentBlock":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.IndentBlock(s)
	case "OutdentBlock":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.OutdentBlock(s)
	case "CycleBlockTodo":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.CycleBlockTodo(s)
	case "ApplySlashOp":
		id, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		op, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return nil, a.ApplySlashOp(id, op)
	case "EnsurePage":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return nil, a.EnsurePage(s)
	case "ResolveWikilink":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return a.ResolveWikilink(s)
	case "ListPagesByProperty":
		k, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		v, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return a.ListPagesByProperty(k, v)
	case "ExportPageHTML":
		p, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		d, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return nil, a.ExportPageHTML(p, d)
	case "GetBacklinks":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return a.GetBacklinks(s)
	case "QueryBlocks":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return a.QueryBlocks(s)
	case "GetAISettings":
		return a.GetAISettings()
	case "SetAISettings":
		var dto bridge.AISettingsDTO
		if len(args) < 1 {
			return nil, fmt.Errorf("missing AI settings")
		}
		if err := json.Unmarshal(args[0], &dto); err != nil {
			return nil, err
		}
		return nil, a.SetAISettings(dto)
	case "AIChat":
		p, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		msg, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		return a.AIChat(p, msg)
	case "GetSyncSettings":
		return a.GetSyncSettings()
	case "SetSyncSettings":
		var dto bridge.SyncSettingsDTO
		if len(args) < 1 {
			return nil, fmt.Errorf("missing sync settings")
		}
		if err := json.Unmarshal(args[0], &dto); err != nil {
			return nil, err
		}
		return nil, a.SetSyncSettings(dto)
	case "SyncVaultWebDAV":
		return nil, a.SyncVaultWebDAV()
	case "SyncVaultS3":
		return nil, a.SyncVaultS3()
	case "ListLANSyncPeers":
		return a.ListLANSyncPeers()
	case "StartLANSyncAdvertise":
		return a.StartLANSyncAdvertise()
	case "StopLANSyncAdvertise":
		a.StopLANSyncAdvertise()
		return nil, nil
	case "PairLANSyncWith":
		host, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		port, err := intArg(args, 1)
		if err != nil {
			return nil, err
		}
		pin, err := strArg(args, 2)
		if err != nil {
			return nil, err
		}
		return nil, a.PairLANSyncWith(host, port, pin)
	case "GetSemanticRelatedForPage":
		p, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		lim, err := intArg(args, 1)
		if err != nil {
			return nil, err
		}
		return a.GetSemanticRelatedForPage(p, lim)
	case "SuggestTagsForBlock":
		s, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		return a.SuggestTagsForBlock(s)
	case "StartAIInlineStream":
		op, err := strArg(args, 0)
		if err != nil {
			return nil, err
		}
		bid, err := strArg(args, 1)
		if err != nil {
			return nil, err
		}
		inst, err := strArg(args, 2)
		if err != nil {
			return nil, err
		}
		return nil, a.StartAIInlineStream(op, bid, inst)
	default:
		return nil, fmt.Errorf("unknown method %q", method)
	}
}
