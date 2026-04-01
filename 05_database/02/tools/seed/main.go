package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	numUsers        = 10_000
	numStamps       = 10
	numHashtags     = 500
	numPosts        = 1_000_000
	numFollows      = 300_000
	genFollows      = 360_000
	numFavorites    = 1_500_000
	genFavorites    = 1_800_000
	numReplies      = 200_000
	numPostStamps   = 500_000
	genPostStamps   = 600_000
	numHashtagPosts = 2_000_000
	genHashtagPosts = 2_400_000
	numHashtagFolls = 50_000
	genHashtagFolls = 60_000
	numBlocks       = 30_000
	genBlocks       = 36_000
	numMutes        = 30_000
	genMutes        = 36_000
)

var stampNames = []string{
	"いいね", "最高", "笑える", "驚き", "悲しい",
	"怒り", "応援", "感謝", "好き", "拍手",
}

func defaultDatabaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://learning:learning@localhost:5432/chapter02"
}

func newID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("uuid.NewV7: %v", err))
	}
	return id
}

func randomTime(rng *rand.Rand, base time.Time, days int) time.Time {
	offset := time.Duration(rng.Int63n(int64(time.Hour * 24 * time.Duration(days))))
	return base.Add(-offset)
}

func step(n int, total int, label string) func(elapsed time.Duration) {
	fmt.Printf("[%d/%d] %-40s", n, total, label)
	start := time.Now()
	return func(_ time.Duration) {
		fmt.Printf("done (%.1fs)\n", time.Since(start).Seconds())
	}
}

func mustCopyFrom(ctx context.Context, pool *pgxpool.Pool, table pgx.Identifier, cols []string, rows [][]interface{}) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acquire conn: %v\n", err)
		os.Exit(1)
	}
	defer conn.Release()

	rowSrc := make([][]interface{}, len(rows))
	copy(rowSrc, rows)

	n, err := conn.CopyFrom(ctx, table, cols, pgx.CopyFromRows(rowSrc))
	if err != nil {
		fmt.Fprintf(os.Stderr, "CopyFrom %s: %v\n", table, err)
		os.Exit(1)
	}
	_ = n
}

func main() {
	ctx := context.Background()
	rng := rand.New(rand.NewSource(42))
	now := time.Now()
	base := now

	pool, err := pgxpool.New(ctx, defaultDatabaseURL())
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v\n", err)
		os.Exit(1)
	}

	const totalSteps = 13

	// ── 1. users ────────────────────────────────────────────────
	{
		done := step(1, totalSteps, fmt.Sprintf("Inserting %d users...", numUsers))
		userIDs := make([]uuid.UUID, numUsers)
		rows := make([][]interface{}, numUsers)
		for i := range numUsers {
			id := newID()
			userIDs[i] = id
			rows[i] = []interface{}{
				id,
				fmt.Sprintf("ユーザー%d", i+1),
				fmt.Sprintf("ユーザー%dの自己紹介文です。", i+1),
				randomTime(rng, base, 365),
			}
		}
		mustCopyFrom(ctx, pool,
			pgx.Identifier{"users"},
			[]string{"id", "display_name", "bio", "created_at"},
			rows,
		)
		done(0)

		// ── 2. stamps ────────────────────────────────────────────
		{
			done2 := step(2, totalSteps, fmt.Sprintf("Inserting %d stamps...", numStamps))
			stampIDs := make([]uuid.UUID, numStamps)
			srows := make([][]interface{}, numStamps)
			for i, name := range stampNames {
				id := newID()
				stampIDs[i] = id
				srows[i] = []interface{}{id, name, randomTime(rng, base, 365)}
			}
			mustCopyFrom(ctx, pool,
				pgx.Identifier{"stamps"},
				[]string{"id", "name", "created_at"},
				srows,
			)
			done2(0)

			// ── 3. hashtags ──────────────────────────────────────
			{
				done3 := step(3, totalSteps, fmt.Sprintf("Inserting %d hashtags...", numHashtags))
				hashtagIDs := make([]uuid.UUID, numHashtags)
				hrows := make([][]interface{}, numHashtags)
				for i := range numHashtags {
					id := newID()
					hashtagIDs[i] = id
					hrows[i] = []interface{}{id, fmt.Sprintf("タグ%d", i+1), randomTime(rng, base, 365)}
				}
				mustCopyFrom(ctx, pool,
					pgx.Identifier{"hashtags"},
					[]string{"id", "name", "created_at"},
					hrows,
				)
				done3(0)

				// ── 4. posts ──────────────────────────────────────
				{
					done4 := step(4, totalSteps, fmt.Sprintf("Inserting %d posts...", numPosts))
					postIDs := make([]uuid.UUID, numPosts)
					prows := make([][]interface{}, numPosts)
					for i := range numPosts {
						id := newID()
						postIDs[i] = id
						prows[i] = []interface{}{
							id,
							userIDs[rng.Intn(numUsers)],
							fmt.Sprintf("投稿%dの内容です。#テスト", i+1),
							randomTime(rng, base, 180),
						}
					}
					mustCopyFrom(ctx, pool,
						pgx.Identifier{"posts"},
						[]string{"id", "user_id", "content", "created_at"},
						prows,
					)
					done4(0)

					// ── 5. follows ──────────────────────────────────
					{
						done5 := step(5, totalSteps, fmt.Sprintf("Inserting ~%d follows...", numFollows))
						seen := make(map[[2]uuid.UUID]struct{}, numFollows)
						frows := make([][]interface{}, 0, numFollows)
						for len(frows) < numFollows {
							u1 := userIDs[rng.Intn(numUsers)]
							u2 := userIDs[rng.Intn(numUsers)]
							if u1 == u2 {
								continue
							}
							key := [2]uuid.UUID{u1, u2}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							frows = append(frows, []interface{}{u1, u2, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"follows"},
							[]string{"user_id", "follow_user_id", "created_at"},
							frows,
						)
						done5(0)
					}

					// ── 6. post_favorites ────────────────────────────
					{
						done6 := step(6, totalSteps, fmt.Sprintf("Inserting ~%d post_favorites...", numFavorites))
						seen := make(map[[2]uuid.UUID]struct{}, numFavorites)
						favrows := make([][]interface{}, 0, numFavorites)
						for len(favrows) < numFavorites {
							postID := postIDs[rng.Intn(numPosts)]
							userID := userIDs[rng.Intn(numUsers)]
							key := [2]uuid.UUID{postID, userID}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							favrows = append(favrows, []interface{}{postID, userID, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"post_favorites"},
							[]string{"post_id", "user_id", "created_at"},
							favrows,
						)
						done6(0)
					}

					// ── 7. post_replies ──────────────────────────────
					{
						done7 := step(7, totalSteps, fmt.Sprintf("Inserting %d post_replies...", numReplies))
						rrows := make([][]interface{}, numReplies)
						for i := range numReplies {
							rrows[i] = []interface{}{
								newID(),
								postIDs[rng.Intn(numPosts)],
								userIDs[rng.Intn(numUsers)],
								fmt.Sprintf("リプライ%dの内容です。", i+1),
								randomTime(rng, base, 180),
							}
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"post_replies"},
							[]string{"id", "post_id", "user_id", "content", "created_at"},
							rrows,
						)
						done7(0)
					}

					// ── 8. post_stamps ───────────────────────────────
					{
						done8 := step(8, totalSteps, fmt.Sprintf("Inserting ~%d post_stamps...", numPostStamps))
						seen := make(map[[3]uuid.UUID]struct{}, numPostStamps)
						psrows := make([][]interface{}, 0, numPostStamps)
						for len(psrows) < numPostStamps {
							postID := postIDs[rng.Intn(numPosts)]
							stampID := stampIDs[rng.Intn(numStamps)]
							userID := userIDs[rng.Intn(numUsers)]
							key := [3]uuid.UUID{postID, stampID, userID}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							psrows = append(psrows, []interface{}{postID, stampID, userID, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"post_stamps"},
							[]string{"post_id", "stamp_id", "user_id", "created_at"},
							psrows,
						)
						done8(0)
					}

					// ── 9. hashtag_posts ─────────────────────────────
					{
						done9 := step(9, totalSteps, fmt.Sprintf("Inserting ~%d hashtag_posts...", numHashtagPosts))
						seen := make(map[[2]uuid.UUID]struct{}, numHashtagPosts)
						hprows := make([][]interface{}, 0, numHashtagPosts)
						for len(hprows) < numHashtagPosts {
							hashtagID := hashtagIDs[rng.Intn(numHashtags)]
							postID := postIDs[rng.Intn(numPosts)]
							key := [2]uuid.UUID{hashtagID, postID}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							hprows = append(hprows, []interface{}{hashtagID, postID, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"hashtag_posts"},
							[]string{"hashtag_id", "post_id", "created_at"},
							hprows,
						)
						done9(0)
					}

					// ── 10. hashtag_follows ──────────────────────────
					{
						done10 := step(10, totalSteps, fmt.Sprintf("Inserting ~%d hashtag_follows...", numHashtagFolls))
						seen := make(map[[2]uuid.UUID]struct{}, numHashtagFolls)
						hfrows := make([][]interface{}, 0, numHashtagFolls)
						for len(hfrows) < numHashtagFolls {
							hashtagID := hashtagIDs[rng.Intn(numHashtags)]
							userID := userIDs[rng.Intn(numUsers)]
							key := [2]uuid.UUID{hashtagID, userID}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							hfrows = append(hfrows, []interface{}{hashtagID, userID, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"hashtag_follows"},
							[]string{"hashtag_id", "user_id", "created_at"},
							hfrows,
						)
						done10(0)
					}

					// ── 11. user_blocks ──────────────────────────────
					{
						done11 := step(11, totalSteps, fmt.Sprintf("Inserting ~%d user_blocks...", numBlocks))
						seen := make(map[[2]uuid.UUID]struct{}, numBlocks)
						brows := make([][]interface{}, 0, numBlocks)
						for len(brows) < numBlocks {
							u1 := userIDs[rng.Intn(numUsers)]
							u2 := userIDs[rng.Intn(numUsers)]
							if u1 == u2 {
								continue
							}
							key := [2]uuid.UUID{u1, u2}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							brows = append(brows, []interface{}{u1, u2, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"user_blocks"},
							[]string{"user_id", "block_user_id", "created_at"},
							brows,
						)
						done11(0)
					}

					// ── 12. user_mutes ───────────────────────────────
					{
						done12 := step(12, totalSteps, fmt.Sprintf("Inserting ~%d user_mutes...", numMutes))
						seen := make(map[[2]uuid.UUID]struct{}, numMutes)
						mrows := make([][]interface{}, 0, numMutes)
						for len(mrows) < numMutes {
							u1 := userIDs[rng.Intn(numUsers)]
							u2 := userIDs[rng.Intn(numUsers)]
							if u1 == u2 {
								continue
							}
							key := [2]uuid.UUID{u1, u2}
							if _, ok := seen[key]; ok {
								continue
							}
							seen[key] = struct{}{}
							mrows = append(mrows, []interface{}{u1, u2, randomTime(rng, base, 180)})
						}
						mustCopyFrom(ctx, pool,
							pgx.Identifier{"user_mutes"},
							[]string{"user_id", "mute_user_id", "created_at"},
							mrows,
						)
						done12(0)
					}

					// ── 13. ANALYZE ──────────────────────────────────
					{
						done13 := step(13, totalSteps, "Running ANALYZE...")
						conn, err := pool.Acquire(ctx)
						if err != nil {
							fmt.Fprintf(os.Stderr, "acquire: %v\n", err)
							os.Exit(1)
						}
						if _, err := conn.Exec(ctx, "ANALYZE"); err != nil {
							conn.Release()
							fmt.Fprintf(os.Stderr, "ANALYZE: %v\n", err)
							os.Exit(1)
						}
						conn.Release()
						done13(0)
					}
				}
			}
		}
	}

	fmt.Println("\nSeed completed successfully.")
}
