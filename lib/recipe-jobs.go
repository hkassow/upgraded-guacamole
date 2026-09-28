package lib

// recipe_jobs rows: a recipe submitted for parsing, tracked until it has been saved.

import (
	"context"
	"encoding/json"
	"log"

	"go-guacamole/db"
	"go-guacamole/models"
)

func CreateRecipeJob(ctx context.Context, job models.RecipeJob) (int, error) {
	imagesJSON, _ := json.Marshal(job.Images)

	var jobID int
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO recipe_jobs (title, text, images, type, parsed, user_id)
         VALUES ($1, $2, $3, $4, FALSE, $5)
         RETURNING id`,
		job.Name, job.Text, imagesJSON, job.Type, job.User_id,
	).Scan(&jobID)

	if err != nil {
		log.Println("Failed to create recipe_job:", err)
		return 0, err
	}

	return jobID, nil
}

func SaveRecipeJobParsedJSON(ctx context.Context, jobID int, parsedJSON []byte) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE recipe_jobs
         SET parsed_json = $1, updated_at = NOW()
         WHERE id = $2`,
		parsedJSON, jobID,
	)

	if err != nil {
		log.Println("Failed to save recipe_job parsed json:", err)
	}

	return err
}

func SaveRecipeJobTranscript(ctx context.Context, jobID int, transcript string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE recipe_jobs
         SET transcript = $1, updated_at = NOW()
         WHERE id = $2`,
		transcript, jobID,
	)

	if err != nil {
		log.Println("Failed to save recipe_job transcript:", err)
	}

	return err
}

func IncrementRecipeJobFailCount(ctx context.Context, jobID int) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE recipe_jobs
         SET fail_count = fail_count + 1, updated_at = NOW()
         WHERE id = $1`,
		jobID,
	)

	if err != nil {
		log.Println("Failed to increment recipe_job fail_count:", err)
	}

	return err
}

func LoadUnparsedRecipeJobs(ctx context.Context) error {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, title, text, images, type, user_id, parsed_json, COALESCE(transcript, '')
         FROM recipe_jobs
         WHERE parsed = FALSE AND fail_count < $1`,
		MaxRecipeJobFailures,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0

	for rows.Next() {
		var job models.RecipeJob
		var imagesJSON []byte
		if err := rows.Scan(&job.ID, &job.Name, &job.Text, &imagesJSON, &job.Type, &job.User_id, &job.ParsedJSON, &job.Transcript); err != nil {
			return err
		}
		if len(imagesJSON) > 0 {
			json.Unmarshal(imagesJSON, &job.Images)
		}

		// Queue the job directly
		RecipeQueue <- job
		count++
	}

	log.Printf("Queued %d unparsed recipe jobs\n", count)
	return nil
}
