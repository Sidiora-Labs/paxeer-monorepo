package account

import "net/http"

func serveV2Owner(w http.ResponseWriter, r *http.Request, core http.HandlerFunc, adapt func([]byte) ([]byte, error)) {
	capture := &captureWriter{}
	core(capture, r)
	if capture.Status >= http.StatusOK && capture.Status < http.StatusMultipleChoices && capture.Status != http.StatusNoContent {
		body, err := adapt(capture.Body)
		if err != nil {
			WriteError(w, err, requestID(r))
			return
		}
		capture.Body = body
		capture.Header().Del("Content-Length")
	}
	writeCaptured(w, capture.capturedResponse)
}

func (h HTTP) preferencesV2Get(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.preferencesGet, func(raw []byte) ([]byte, error) { return AdaptV2Preferences(raw, false) })
}

func (h HTTP) preferencesV2Patch(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.preferencesPatch, func(raw []byte) ([]byte, error) { return AdaptV2Preferences(raw, true) })
}

func (h HTTP) deviceV2Put(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.devicePut, AdaptV2DeviceRegister)
}

func (h HTTP) deviceV2Delete(w http.ResponseWriter, r *http.Request) {
	h.deviceDelete(w, r)
}

func (h HTTP) exportV2Post(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.exportPost, func(raw []byte) ([]byte, error) { return AdaptV2AccountJob(raw, "export", false) })
}

func (h HTTP) exportV2Get(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.exportGet, func(raw []byte) ([]byte, error) { return AdaptV2AccountJob(raw, "export", true) })
}

func (h HTTP) deletionV2Post(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.deletionPost, func(raw []byte) ([]byte, error) { return AdaptV2AccountJob(raw, "deletion", false) })
}

func (h HTTP) deletionV2Get(w http.ResponseWriter, r *http.Request) {
	serveV2Owner(w, r, h.deletionGet, func(raw []byte) ([]byte, error) { return AdaptV2AccountJob(raw, "deletion", true) })
}
