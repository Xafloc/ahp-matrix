package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"github.com/Xafloc/ahp-matrix/internal/model"
)

// packetSlot is where the judging page expects its decision.
var packetSlot = []byte(`<script type="application/json" id="ahp-packet"></script>`)

// invitePage serves the self-contained judging page with one person's copy
// inside it, for them to open in any browser with nothing installed.
func (s *server) invitePage(w http.ResponseWriter, r *http.Request) {
	p, ok := s.load(w, r)
	if !ok {
		return
	}
	packet, err := model.Packet(p, r.URL.Query().Get("evaluator"))
	if errors.Is(err, model.ErrUnknownEvaluator) {
		writeError(w, http.StatusBadRequest, "unknown-evaluator", err.Error(), nil)
		return
	} else if err != nil {
		s.internalError(w, err)
		return
	}
	var page []byte
	if s.cfg.UI != nil {
		page, _ = fs.ReadFile(s.cfg.UI, "judge.html")
	}
	if !bytes.Contains(page, packetSlot) {
		writeError(w, http.StatusServiceUnavailable, "judge-page-missing", "this build has no judging page; build the web UI first", nil)
		return
	}
	// json.Marshal escapes <, > and &, so nothing in a name can close the script element.
	data, err := json.Marshal(packet)
	if err != nil {
		s.internalError(w, err)
		return
	}
	slot := append(append([]byte(`<script type="application/json" id="ahp-packet">`), data...), []byte(`</script>`)...)
	attachment(w, model.Slug(p.Name)+"-for-"+model.Slug(packet.Evaluators[0].Name)+".html", "text/html; charset=utf-8")
	_, _ = w.Write(bytes.Replace(page, packetSlot, slot, 1))
}
