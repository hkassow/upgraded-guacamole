package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCallDeepInfraStream(t *testing.T) {
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": keep-alive comment\n\n")
		writeEvent(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{"role": "assistant"}}}})
		writeStreamChunks(w, "½ cup sugar, 2¾ cups flour", "stop")
	})

	got, err := callDeepInfra(diChatRequest{Model: deepInfraTextModel})
	if err != nil {
		t.Fatalf("callDeepInfra: %v", err)
	}
	if got != "½ cup sugar, 2¾ cups flour" {
		t.Errorf("content = %q", got)
	}
}

func TestCallDeepInfraCutOffReplyIsReturned(t *testing.T) {
	// finish_reason "length" is logged; the (incomplete) text is still returned for the caller to reject
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		writeStreamChunks(w, `{"steps": {"main": ["Mix`, "length")
	})

	got, err := callDeepInfra(diChatRequest{Model: deepInfraTextModel})
	if err != nil || got != `{"steps": {"main": ["Mix` {
		t.Errorf("got %q, %v", got, err)
	}
}

// waitForClientToGiveUp blocks a fake handler until the client disconnects. The body has to be read
// first: Go's server only notices a closed connection once the request body is consumed. The cap
// makes a regression fail the test instead of hanging the whole run.
func waitForClientToGiveUp(r *http.Request) {
	io.Copy(io.Discard, r.Body)
	select {
	case <-r.Context().Done():
	case <-time.After(5 * time.Second):
	}
}

func TestCallDeepInfraNoDataBeforeTimeout(t *testing.T) {
	shortDeepInfraTimeouts(t)
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		waitForClientToGiveUp(r) // never responds
	})

	start := time.Now()
	_, err := callDeepInfra(diChatRequest{Model: deepInfraTextModel})
	if err == nil || !strings.Contains(err.Error(), "no data received for 100ms") {
		t.Errorf("error = %v, want a first-data timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s, should give up after the first-data timeout", elapsed)
	}
}

func TestCallDeepInfraStreamStalls(t *testing.T) {
	shortDeepInfraTimeouts(t)
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeEvent(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{"content": `{"steps"`}}}})
		waitForClientToGiveUp(r) // then goes quiet
	})

	_, err := callDeepInfra(diChatRequest{Model: deepInfraTextModel})
	if err == nil || !strings.Contains(err.Error(), "no data received for 100ms") {
		t.Errorf("error = %v, want an idle timeout", err)
	}
}

func TestCallDeepInfraSlowButSteadyStreamSucceeds(t *testing.T) {
	shortDeepInfraTimeouts(t)
	// total time is well over the idle timeout, but data keeps arriving, so it must not be cut off
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 6; i++ {
			writeEvent(w, map[string]any{"choices": []map[string]any{{"delta": map[string]any{"content": "x"}}}})
			time.Sleep(50 * time.Millisecond)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	got, err := callDeepInfra(diChatRequest{Model: deepInfraTextModel})
	if err != nil || got != "xxxxxx" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseRecipeImageCallDeepInfra(t *testing.T) {
	var requests []diChatRequest
	fakeDeepInfra(t, func(w http.ResponseWriter, r *http.Request) {
		var req diChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		requests = append(requests, req)

		if len(requests) == 1 {
			writeChatCompletion(w, "Honey soy chicken\n500g chicken thighs\n...")
			return
		}
		writeChatCompletion(w, sampleModelJSON)
	})

	parsed, err := ParseRecipeImageCallDeepInfra([]string{"aW1hZ2Ux", "aW1hZ2Uy"})
	if err != nil {
		t.Fatalf("ParseRecipeImageCallDeepInfra: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("DeepInfra calls = %d, want 2 (transcribe, then extract)", len(requests))
	}
	if requests[0].Model != deepInfraVisionModel {
		t.Errorf("first call model = %q, want vision model", requests[0].Model)
	}
	// user message content is [image, image, text] for two images
	if parts, ok := requests[0].Messages[1].Content.([]any); !ok || len(parts) != 3 {
		t.Errorf("vision request content = %#v, want 3 parts", requests[0].Messages[1].Content)
	}
	if requests[1].Model != deepInfraTextModel || requests[1].Messages[1].Content != "Honey soy chicken\n500g chicken thighs\n..." {
		t.Errorf("second call should send the transcript to the text model, got %+v", requests[1])
	}
	if len(parsed.Ingredients) != 4 {
		t.Errorf("ingredients = %d, want 4", len(parsed.Ingredients))
	}
}

func TestParseRecipeImageCallDeepInfraNoImages(t *testing.T) {
	calls := fakeDeepInfra(t, modelMustNotBeCalled(t))

	if _, err := ParseRecipeImageCallDeepInfra(nil); err == nil {
		t.Error("expected an error for no images")
	}
	if calls.Load() != 0 {
		t.Errorf("DeepInfra calls = %d, want 0", calls.Load())
	}
}
