package lib

// Background worker that parses queued recipe jobs (text or photos) and saves them.

import (
	"context"
	"encoding/json"
	"fmt"
	"go-guacamole/models"
	"log"
)

var RecipeQueue = make(chan models.RecipeJob, 100)

// jobs that have failed this many times are no longer loaded for retry
const MaxRecipeJobFailures = 3

// usable reports whether a parse result has enough in it to be worth saving/reusing.
func (p *RecipeParsed) usable() bool {
	return p != nil && (len(p.Steps) > 0 || len(p.Ingredients) > 0)
}

// cachedRecipeParse returns the parse result stored on a job from a previous run, or nil if
// there isn't one or it can't be used.
func cachedRecipeParse(jobID int, raw []byte) *RecipeParsed {
	if len(raw) == 0 {
		return nil
	}

	parsed := &RecipeParsed{}
	if err := json.Unmarshal(raw, parsed); err != nil || !parsed.usable() {
		log.Printf("recipe job %d: ignoring stored parsed_json (err: %v)", jobID, err)
		return nil
	}

	return parsed
}

// QueueRecipeJob saves the job to recipe_jobs and then hands it to the worker. Saving first means a
// job still waiting in the queue isn't lost if the server restarts - LoadUnparsedRecipeJobs picks it
// back up on startup.
func QueueRecipeJob(ctx context.Context, job models.RecipeJob) error {
	jobID, err := CreateRecipeJob(ctx, job)
	if err != nil {
		return err
	}
	job.ID = jobID

	RecipeQueue <- job
	return nil
}

func StartRecipeWorker() {
	go func() {
		for job := range RecipeQueue {
			if err := ProcessRecipeJob(context.Background(), job); err != nil {
				log.Printf("Recipe job %q failed: %v", job.Name, err)
				continue
			}
			log.Println("Recipe saved successfully:", job.Name)
		}
	}()
}

// ProcessRecipeJob parses one queued recipe (reusing a stored parse result if there is one) and saves
// it. Failures bump the job's fail_count so it stops being retried after MaxRecipeJobFailures.
func ProcessRecipeJob(ctx context.Context, job models.RecipeJob) error {
	log.Println("Processing recipe:", job.Name)

	jobID := job.ID
	if jobID == 0 {
		var err error
		jobID, err = CreateRecipeJob(ctx, job)
		if err != nil {
			return fmt.Errorf("creating recipe job: %w", err)
		}
	}

	// reuse the model's output from a previous attempt instead of paying for another call
	parsed := cachedRecipeParse(jobID, job.ParsedJSON)
	if parsed != nil {
		log.Println("Reusing stored parse result for recipe:", job.Name)
	} else {
		var err error

		switch job.Type {
		case "image":
			//parsed, err = ParseRecipeImageCall(job.Images)
			transcript := job.Transcript
			if transcript != "" {
				log.Println("Reusing stored transcript for recipe:", job.Name)
			} else if transcript, err = TranscribeRecipeImages(job.Images); err == nil {
				_ = SaveRecipeJobTranscript(ctx, jobID, transcript)
			}
			if err == nil {
				parsed, err = extractRecipeJSON(transcript)
			}
		default: // "text"
			//parsed, err = ParseRecipeCall(job.Text)
			parsed, err = ParseRecipeCallDeepInfra(job.Text)
		}

		if err == nil && !parsed.usable() {
			err = fmt.Errorf("model returned no steps or ingredients")
		}
		if err != nil {
			_ = IncrementRecipeJobFailCount(ctx, jobID)
			return fmt.Errorf("parsing recipe: %w", err)
		}

		if parsedJSON, err := json.Marshal(parsed); err == nil {
			_ = SaveRecipeJobParsedJSON(ctx, jobID, parsedJSON)
		} else {
			log.Println("Error marshaling parsed recipe:", err)
		}
	}

	if err := SaveParsedRecipe(ctx, job.Name, job.User_id, parsed, jobID); err != nil {
		_ = IncrementRecipeJobFailCount(ctx, jobID)
		return fmt.Errorf("saving recipe: %w", err)
	}

	return nil
}
