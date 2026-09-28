package lib

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	maxTagLength     = 30
	maxTagsPerRecipe = 20
)

// ErrInvalidTags is returned when a tag list can't be saved (too long a tag, too many tags).
var ErrInvalidTags = errors.New("invalid tags")

// NormalizeTags lowercases, trims and de-duplicates tags ("  Vegan ", "#vegan" -> "vegan"), keeping
// the first-seen order and skipping empty ones.
func NormalizeTags(tags []string) ([]string, error) {
	seen := make(map[string]bool, len(tags))
	normalized := make([]string, 0, len(tags))

	for _, tag := range tags {
		tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
		tag = strings.ToLower(strings.Join(strings.Fields(tag), " "))
		if tag == "" || seen[tag] {
			continue
		}
		if utf8.RuneCountInString(tag) > maxTagLength {
			return nil, fmt.Errorf("%w: %q is longer than %d characters", ErrInvalidTags, tag, maxTagLength)
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}

	if len(normalized) > maxTagsPerRecipe {
		return nil, fmt.Errorf("%w: a recipe can have at most %d tags", ErrInvalidTags, maxTagsPerRecipe)
	}
	return normalized, nil
}

// setRecipeTags replaces a recipe's tags with tags (already normalized), creating any tag that
// doesn't exist yet. Runs inside the caller's transaction.
func setRecipeTags(ctx context.Context, tx pgx.Tx, recipeID int, tags []string) error {
	if len(tags) > 0 {
		_, err := tx.Exec(ctx,
			`INSERT INTO tags (name) SELECT unnest($1::text[]) ON CONFLICT (name) DO NOTHING`,
			tags,
		)
		if err != nil {
			return fmt.Errorf("create tags: %w", err)
		}
	}

	_, err := tx.Exec(ctx,
		`DELETE FROM recipe_tags
		 WHERE recipe_id = $1
		   AND tag_id NOT IN (SELECT id FROM tags WHERE name = ANY($2::text[]))`,
		recipeID, tags,
	)
	if err != nil {
		return fmt.Errorf("remove recipe tags: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO recipe_tags (recipe_id, tag_id)
		 SELECT $1, id FROM tags WHERE name = ANY($2::text[])
		 ON CONFLICT DO NOTHING`,
		recipeID, tags,
	)
	if err != nil {
		return fmt.Errorf("add recipe tags: %w", err)
	}
	return nil
}
