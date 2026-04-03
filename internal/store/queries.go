package store

import (
	"context"
	"fmt"

	"github.com/bitravens/paravizor/v1/internal/store/db"
)

// InsertDomain upserts a domain and returns its ID.
// On conflict the updated_at timestamp is refreshed.
func (s *Store) InsertDomain(ctx context.Context, name, source string, batchID *int64) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertOrTouchDomain(ctx, db.InsertOrTouchDomainParams{
			Name:    name,
			Source:  source,
			BatchID: batchID,
		}); err != nil {
			return fmt.Errorf("upsert domain: %w", err)
		}
		row, err := q.GetDomainByName(ctx, name)
		if err != nil {
			return fmt.Errorf("fetch domain id: %w", err)
		}
		id = row.ID
		return nil
	})
	return id, err
}

// InsertDomainsBatch upserts multiple domains in a single transaction.
// Returns the count of rows passed in (not inserted — upserts always succeed).
func (s *Store) InsertDomainsBatch(ctx context.Context, names []string, source string, batchID *int64) (int, error) {
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		for _, name := range names {
			if err := q.InsertOrTouchDomain(ctx, db.InsertOrTouchDomainParams{
				Name:    name,
				Source:  source,
				BatchID: batchID,
			}); err != nil {
				return fmt.Errorf("upsert domain %q: %w", name, err)
			}
		}
		return nil
	})
	return len(names), err
}

// InsertURL upserts a URL and returns its ID.
func (s *Store) InsertURL(ctx context.Context, fullURL, source string, domainID *int64, batchID *int64) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertOrTouchURL(ctx, db.InsertOrTouchURLParams{
			FullUrl:  fullURL,
			Source:   source,
			DomainID: domainID,
			BatchID:  batchID,
		}); err != nil {
			return fmt.Errorf("upsert url: %w", err)
		}
		row, err := q.GetURLByFullURL(ctx, fullURL)
		if err != nil {
			return fmt.Errorf("fetch url id: %w", err)
		}
		id = row.ID
		return nil
	})
	return id, err
}

// InsertFinding inserts a finding and returns its ID.
func (s *Store) InsertFinding(ctx context.Context, f *db.Finding) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertFinding(ctx, db.InsertFindingParams{
			DomainID:    f.DomainID,
			UrlID:       f.UrlID,
			Scanner:     f.Scanner,
			Severity:    f.Severity,
			Title:       f.Title,
			Description: f.Description,
			Evidence:    f.Evidence,
		}); err != nil {
			return fmt.Errorf("insert finding: %w", err)
		}
		// SQLite last_insert_rowid() via the write connection.
		if err := s.writeDB.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id); err != nil {
			return fmt.Errorf("fetch finding id: %w", err)
		}
		return nil
	})
	return id, err
}

// GetDomains returns a page of domains; pass liveOnly=true to filter.
func (s *Store) GetDomains(ctx context.Context, liveOnly bool, limit, offset int) ([]db.Domain, error) {
	if liveOnly {
		return s.rq.GetLiveDomains(ctx, db.GetLiveDomainsParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
	}
	return s.rq.GetDomains(ctx, db.GetDomainsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
}

// CountDomains returns the total number of domains.
func (s *Store) CountDomains(ctx context.Context) (int64, error) {
	return s.rq.CountDomains(ctx)
}

// GetURLs returns a page of URLs.
func (s *Store) GetURLs(ctx context.Context, limit, offset int) ([]db.Url, error) {
	return s.rq.GetURLs(ctx, db.GetURLsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
}

// CountURLs returns the total number of URLs.
func (s *Store) CountURLs(ctx context.Context) (int64, error) {
	return s.rq.CountURLs(ctx)
}

// GetFindings returns a page of findings.
func (s *Store) GetFindings(ctx context.Context, limit, offset int) ([]db.Finding, error) {
	return s.rq.GetFindings(ctx, db.GetFindingsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
}

// CountFindings returns the total number of findings.
func (s *Store) CountFindings(ctx context.Context) (int64, error) {
	return s.rq.CountFindings(ctx)
}

// InsertScopeRule inserts a scope rule and returns its ID.
func (s *Store) InsertScopeRule(ctx context.Context, pattern, ruleType string, inScope bool) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertScopeRule(ctx, db.InsertScopeRuleParams{
			Pattern: pattern,
			Type:    ruleType,
			InScope: inScope,
		}); err != nil {
			return fmt.Errorf("insert scope rule: %w", err)
		}
		if err := s.writeDB.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id); err != nil {
			return fmt.Errorf("fetch scope rule id: %w", err)
		}
		return nil
	})
	return id, err
}

// GetScopeRules returns all scope rules.
func (s *Store) GetScopeRules(ctx context.Context) ([]db.ScopeRule, error) {
	return s.rq.GetScopeRules(ctx)
}

// DeleteScopeRule deletes a scope rule by pattern.
func (s *Store) DeleteScopeRule(ctx context.Context, pattern string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		return q.DeleteScopeRuleByPattern(ctx, pattern)
	})
}

// SetPipelineState upserts a pipeline_state entry.
// Uses INSERT OR IGNORE to create if absent, then UPDATE to set fields.
func (s *Store) SetPipelineState(ctx context.Context, ps *db.PipelineState) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertOrIgnorePipelineState(ctx, db.InsertOrIgnorePipelineStateParams{
			ItemType: ps.ItemType,
			ItemID:   ps.ItemID,
			NodeID:   ps.NodeID,
		}); err != nil {
			return fmt.Errorf("ensure pipeline state row: %w", err)
		}
		return q.UpdatePipelineState(ctx, db.UpdatePipelineStateParams{
			Status:      ps.Status,
			StartedAt:   ps.StartedAt,
			CompletedAt: ps.CompletedAt,
			Error:       ps.Error,
			ItemID:      ps.ItemID,
			ItemType:    ps.ItemType,
			NodeID:      ps.NodeID,
		})
	})
}

// GetPendingItems returns pipeline items pending at a given node.
func (s *Store) GetPendingItems(ctx context.Context, nodeID, itemType string, limit int) ([]db.PipelineState, error) {
	return s.rq.GetPendingItems(ctx, db.GetPendingItemsParams{
		NodeID:   nodeID,
		ItemType: itemType,
		Limit:    int64(limit),
	})
}

// ResetProcessingItems marks all 'processing' pipeline items back to 'pending'.
func (s *Store) ResetProcessingItems(ctx context.Context) (int64, error) {
	var affected int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		res, err := q.ResetProcessingItems(ctx)
		if err != nil {
			return err
		}
		affected, err = res.RowsAffected()
		return err
	})
	return affected, err
}

// HasInterruptedSession returns true if any pipeline items are stuck in 'processing'.
func (s *Store) HasInterruptedSession(ctx context.Context) (bool, error) {
	count, err := s.rq.HasInterruptedSession(ctx)
	return count > 0, err
}

// InsertBatch creates a new batch record in 'processing' state and returns its ID.
func (s *Store) InsertBatch(ctx context.Context, nodeID string, itemCount int) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertBatch(ctx, db.InsertBatchParams{
			NodeID:    nodeID,
			ItemCount: int64(itemCount),
		}); err != nil {
			return err
		}
		return s.writeDB.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id)
	})
	return id, err
}

// CompleteBatch marks a batch as completed or failed.
func (s *Store) CompleteBatch(ctx context.Context, batchID int64, status string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		return q.CompleteBatch(ctx, db.CompleteBatchParams{
			Status: status,
			ID:     batchID,
		})
	})
}

// InsertProcess creates a new process record and returns its ID.
func (s *Store) InsertProcess(ctx context.Context, p *db.Process) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertProcess(ctx, db.InsertProcessParams{
			Name:       p.Name,
			Command:    p.Command,
			Pid:        p.Pid,
			NodeID:     p.NodeID,
			BatchID:    p.BatchID,
			StdoutPath: p.StdoutPath,
			StderrPath: p.StderrPath,
		}); err != nil {
			return err
		}
		return s.writeDB.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id)
	})
	return id, err
}

// CompleteProcess marks a process as completed with a final status and exit code.
func (s *Store) CompleteProcess(ctx context.Context, processID int64, status string, exitCode int) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		ec := int64(exitCode)
		return q.CompleteProcess(ctx, db.CompleteProcessParams{
			Status:   status,
			ExitCode: &ec,
			ID:       processID,
		})
	})
}

// InsertURLFlag upserts a url_flag, ignoring if the exact (url_id, flag_type, flag_value) triple exists.
func (s *Store) InsertURLFlag(ctx context.Context, urlID int64, flagType string, flagValue *string, source string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		return q.InsertOrIgnoreURLFlag(ctx, db.InsertOrIgnoreURLFlagParams{
			UrlID:     urlID,
			FlagType:  flagType,
			FlagValue: flagValue,
			Source:    source,
		})
	})
}

// UpsertTechStack upserts a technology detection for a domain.
func (s *Store) UpsertTechStack(ctx context.Context, domainID int64, tech string, version, category *string, source string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertOrIgnoreTechStack(ctx, db.InsertOrIgnoreTechStackParams{
			DomainID:   domainID,
			Technology: tech,
			Version:    version,
			Category:   category,
			Source:     source,
		}); err != nil {
			return err
		}
		return q.UpdateTechStack(ctx, db.UpdateTechStackParams{
			Version:    version,
			Category:   category,
			Source:     source,
			DomainID:   domainID,
			Technology: tech,
		})
	})
}

// UpsertIP inserts an IP address if it doesn't exist and returns its ID.
func (s *Store) UpsertIP(ctx context.Context, address string) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertOrIgnoreIP(ctx, address); err != nil {
			return err
		}
		row, err := q.GetIPByAddress(ctx, address)
		if err != nil {
			return err
		}
		id = row.ID
		return nil
	})
	return id, err
}

// UpsertPort upserts an open port on an IP.
func (s *Store) UpsertPort(ctx context.Context, ipID int64, port int, protocol string, service, banner *string, source string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		p := int64(port)
		if err := q.InsertOrIgnorePort(ctx, db.InsertOrIgnorePortParams{
			IpID:     ipID,
			Port:     p,
			Protocol: protocol,
			Service:  service,
			Banner:   banner,
			Source:   source,
		}); err != nil {
			return err
		}
		return q.UpdatePort(ctx, db.UpdatePortParams{
			Service:  service,
			Banner:   banner,
			Source:   source,
			IpID:     ipID,
			Port:     p,
			Protocol: protocol,
		})
	})
}

// UpsertDNSRecord upserts a DNS record for a domain.
func (s *Store) UpsertDNSRecord(ctx context.Context, domainID int64, recordType, value string, ttl *int64, source string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertOrIgnoreDNSRecord(ctx, db.InsertOrIgnoreDNSRecordParams{
			DomainID:   domainID,
			RecordType: recordType,
			Value:      value,
			Ttl:        ttl,
			Source:     source,
		}); err != nil {
			return err
		}
		return q.UpdateDNSRecord(ctx, db.UpdateDNSRecordParams{
			Ttl:        ttl,
			Source:     source,
			DomainID:   domainID,
			RecordType: recordType,
			Value:      value,
		})
	})
}

// InsertDownloadedFile records a downloaded file and returns its ID.
func (s *Store) InsertDownloadedFile(ctx context.Context, urlID int64, filePath, fileType string, sizeBytes *int64, sha256 *string) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertDownloadedFile(ctx, db.InsertDownloadedFileParams{
			UrlID:     urlID,
			FilePath:  filePath,
			FileType:  fileType,
			SizeBytes: sizeBytes,
			Sha256:    sha256,
		}); err != nil {
			return err
		}
		return s.writeDB.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id)
	})
	return id, err
}

// HeartbeatProcess updates the last_heartbeat timestamp for a process.
func (s *Store) HeartbeatProcess(ctx context.Context, processID int64) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		return q.HeartbeatProcess(ctx, processID)
	})
}

// GetRunningProcessPIDs returns a list of PIDs for all currently running processes.
func (s *Store) GetRunningProcessPIDs(ctx context.Context) ([]int64, error) {
	var rawPids []*int64
	var err error

	rawPids, err = s.rq.GetRunningProcessPIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get running process pids: %w", err)
	}

	var pids []int64
	for _, pid := range rawPids {
		if pid != nil {
			pids = append(pids, *pid)
		}
	}

	return pids, nil
}

// ============================================================
// Notes
// ============================================================

func (s *Store) InsertNote(ctx context.Context, content string) (int64, error) {
	var id int64
	err := s.WriteTx(ctx, func(q *db.Queries) error {
		if err := q.InsertNote(ctx, content); err != nil {
			return err
		}
		return s.writeDB.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id)
	})
	return id, err
}

func (s *Store) GetNotes(ctx context.Context) ([]db.Note, error) {
	return s.Q().GetNotes(ctx)
}

func (s *Store) UpdateNote(ctx context.Context, id int64, content string) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		return q.UpdateNote(ctx, db.UpdateNoteParams{
			ID:      id,
			Content: content,
		})
	})
}

func (s *Store) DeleteNote(ctx context.Context, id int64) error {
	return s.WriteTx(ctx, func(q *db.Queries) error {
		return q.DeleteNote(ctx, id)
	})
}
