/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_archive_participants.go - Raw dataset chunk storage and participant synchronization helpers.
##
*/

package prisma

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"overdrive/resources/db"
)

func (s *RaceArchiveStore) storeDatasets(ctx context.Context, archiveID string, datasets map[string][]map[string]any) error {
	queries, err := s.buildDatasetChunkQueries(archiveID, datasets)
	if err != nil {
		return err
	}

	for _, query := range queries {
		if err := s.client.Prisma.Transaction(query).Exec(ctx); err != nil {
			return fmt.Errorf("create dataset chunk transaction: %w", err)
		}
	}

	return nil
}

// buildDatasetChunkQueries converts raw datasets into chunked archive insert queries.
func (s *RaceArchiveStore) buildDatasetChunkQueries(archiveID string, datasets map[string][]map[string]any) ([]db.PrismaTransaction, error) {
	keys := make([]string, 0, len(datasets))
	for key := range datasets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	queries := make([]db.PrismaTransaction, 0, len(keys))
	for _, key := range keys {
		chunks := chunkRows(nonNilRows(datasets[key]), rawDatasetChunkSize)
		for chunkIndex, chunk := range chunks {
			rowsJSON, err := marshalPrismaJSON(chunk)
			if err != nil {
				return nil, fmt.Errorf("marshal dataset %q chunk %d: %w", key, chunkIndex, err)
			}

			queries = append(queries, s.client.RaceDatasetChunk.CreateOne(
				db.RaceDatasetChunk.Dataset.Set(key),
				db.RaceDatasetChunk.Rows.Set(rowsJSON),
				db.RaceDatasetChunk.Archive.Link(db.RaceArchive.ID.Equals(archiveID)),
				db.RaceDatasetChunk.ChunkIndex.Set(chunkIndex),
				db.RaceDatasetChunk.RowCount.Set(len(chunk)),
			).Tx())
		}
	}

	return queries, nil
}

// syncParticipants upserts teams and drivers from the provider drivers dataset.
func (s *RaceArchiveStore) syncParticipants(ctx context.Context, providerID string, rows []map[string]any) error {
	for _, row := range rows {
		driverNumber := readIntField(row, "driver_number")
		if driverNumber <= 0 {
			continue
		}

		team, err := s.ensureTeam(ctx, providerID, row)
		if err != nil {
			return fmt.Errorf("ensure team for driver_number=%d: %w", driverNumber, err)
		}

		if err := s.ensureDriver(ctx, providerID, team.ID, row); err != nil {
			return fmt.Errorf("ensure driver_number=%d: %w", driverNumber, err)
		}
	}

	return nil
}

// ensureTeam upserts a provider team using the OpenF1 driver payload.
func (s *RaceArchiveStore) ensureTeam(ctx context.Context, providerID string, row map[string]any) (*db.TeamModel, error) {
	teamName := fallbackString(readStringField(row, "team_name"), "Unknown")
	externalKey := teamExternalKey(teamName)

	team, err := s.client.Team.FindFirst(
		db.Team.ProviderID.Equals(providerID),
		db.Team.ExternalKey.Equals(externalKey),
	).Exec(ctx)
	if err != nil && !db.IsErrNotFound(err) {
		return nil, fmt.Errorf("find team: %w", err)
	}

	params := []db.TeamSetParam{
		db.Team.Name.Set(teamName),
		db.Team.ExternalKey.Set(externalKey),
		db.Team.Code.SetIfPresent(optionalString(readStringField(row, "team_code"))),
		db.Team.ColorHex.SetIfPresent(optionalString(normalizeColor(readStringField(row, "team_colour")))),
	}

	if err == nil {
		updated, updateErr := s.client.Team.FindUnique(
			db.Team.ID.Equals(team.ID),
		).Update(params...).Exec(ctx)
		if updateErr != nil {
			return nil, fmt.Errorf("update team: %w", updateErr)
		}
		return updated, nil
	}

	created, createErr := s.client.Team.CreateOne(
		db.Team.Name.Set(teamName),
		db.Team.Provider.Link(db.Provider.ID.Equals(providerID)),
		params[1:]...,
	).Exec(ctx)
	if createErr != nil {
		return nil, fmt.Errorf("create team: %w", createErr)
	}
	return created, nil
}

// ensureDriver upserts a provider driver using the OpenF1 driver payload.
func (s *RaceArchiveStore) ensureDriver(ctx context.Context, providerID, teamID string, row map[string]any) error {
	driverNumber := readIntField(row, "driver_number")
	if driverNumber <= 0 {
		return nil
	}

	displayName := fallbackString(
		readFirstStringField(row, "full_name", "broadcast_name", "name"),
		fmt.Sprintf("Driver %d", driverNumber),
	)
	externalKey := strconv.Itoa(driverNumber)
	firstName := optionalString(readStringField(row, "first_name"))
	lastName := optionalString(readStringField(row, "last_name"))
	code := optionalString(readFirstStringField(row, "name_acronym", "driver_code", "broadcast_name"))
	countryCode := optionalString(readStringField(row, "country_code"))

	driver, err := s.client.Driver.FindFirst(
		db.Driver.ProviderID.Equals(providerID),
		db.Driver.ExternalKey.Equals(externalKey),
	).Exec(ctx)
	if err != nil && !db.IsErrNotFound(err) {
		return fmt.Errorf("find driver: %w", err)
	}

	params := []db.DriverSetParam{
		db.Driver.DisplayName.Set(displayName),
		db.Driver.Number.Set(driverNumber),
		db.Driver.ExternalKey.Set(externalKey),
		db.Driver.FirstName.SetIfPresent(firstName),
		db.Driver.LastName.SetIfPresent(lastName),
		db.Driver.Code.SetIfPresent(code),
		db.Driver.CountryCode.SetIfPresent(countryCode),
		db.Driver.Team.Link(db.Team.ID.Equals(teamID)),
	}

	if err == nil {
		_, updateErr := s.client.Driver.FindUnique(
			db.Driver.ID.Equals(driver.ID),
		).Update(params...).Exec(ctx)
		if updateErr != nil {
			return fmt.Errorf("update driver: %w", updateErr)
		}
		return nil
	}

	_, createErr := s.client.Driver.CreateOne(
		db.Driver.DisplayName.Set(displayName),
		db.Driver.Number.Set(driverNumber),
		db.Driver.Provider.Link(db.Provider.ID.Equals(providerID)),
		db.Driver.Team.Link(db.Team.ID.Equals(teamID)),
		db.Driver.ExternalKey.Set(externalKey),
		db.Driver.FirstName.SetIfPresent(firstName),
		db.Driver.LastName.SetIfPresent(lastName),
		db.Driver.Code.SetIfPresent(code),
		db.Driver.CountryCode.SetIfPresent(countryCode),
	).Exec(ctx)
	if createErr != nil {
		return fmt.Errorf("create driver: %w", createErr)
	}

	return nil
}
