package items

import "fmt"

// ItemType identifies the category of a recon item.
type ItemType string

const (
	TypeDomain    ItemType = "domain"
	TypeURL       ItemType = "url"
	TypeIP        ItemType = "ip"
	TypePort      ItemType = "port"
	TypeDNSRecord ItemType = "dns_record"
	TypeFinding   ItemType = "finding"
	TypeFile      ItemType = "file"
)

// ScopeTarget is the value used for scope-rule matching (usually a hostname or URL).
type ScopeTarget string

// Item is the interface every recon item must implement.
// All items are discovered or produced by a tool execution and flow through the pipeline.
type Item interface {
	// Type returns the category of this item.
	Type() ItemType
	ItemID() int64
	Source() string
	ScopeTarget() ScopeTarget
	Value() string
}

type DomainItem struct {
	ID         int64
	Name       string
	SourceName string
}

func (d DomainItem) Type() ItemType           { return TypeDomain }
func (d DomainItem) ItemID() int64            { return d.ID }
func (d DomainItem) Source() string           { return d.SourceName }
func (d DomainItem) ScopeTarget() ScopeTarget { return ScopeTarget(d.Name) }
func (d DomainItem) Value() string            { return d.Name }

type URLItem struct {
	ID         int64
	FullURL    string
	SourceName string
}

func (u URLItem) Type() ItemType           { return TypeURL }
func (u URLItem) ItemID() int64            { return u.ID }
func (u URLItem) Source() string           { return u.SourceName }
func (u URLItem) ScopeTarget() ScopeTarget { return ScopeTarget(u.FullURL) }
func (u URLItem) Value() string            { return u.FullURL }

type IPItem struct {
	ID         int64
	Address    string
	SourceName string
}

func (i IPItem) Type() ItemType           { return TypeIP }
func (i IPItem) ItemID() int64            { return i.ID }
func (i IPItem) Source() string           { return i.SourceName }
func (i IPItem) ScopeTarget() ScopeTarget { return ScopeTarget(i.Address) }
func (i IPItem) Value() string            { return i.Address }

type PortItem struct {
	ID         int64
	Host       string
	Port       int
	Protocol   string
	SourceName string
}

func (p PortItem) Type() ItemType           { return TypePort }
func (p PortItem) ItemID() int64            { return p.ID }
func (p PortItem) Source() string           { return p.SourceName }
func (p PortItem) ScopeTarget() ScopeTarget { return ScopeTarget(p.Host) }
func (p PortItem) Value() string {
	if p.Protocol != "" {
		return fmt.Sprintf("%s:%d/%s", p.Host, p.Port, p.Protocol)
	}
	return fmt.Sprintf("%s:%d", p.Host, p.Port)
}

type DNSRecordItem struct {
	ID          int64
	Name        string // e.g. "sub.example.com"
	RecordType  string // A, AAAA, CNAME, MX, TXT…
	RecordValue string
	SourceName  string
}

func (d DNSRecordItem) Type() ItemType           { return TypeDNSRecord }
func (d DNSRecordItem) ItemID() int64            { return d.ID }
func (d DNSRecordItem) Source() string           { return d.SourceName }
func (d DNSRecordItem) ScopeTarget() ScopeTarget { return ScopeTarget(d.Name) }
func (d DNSRecordItem) Value() string            { return d.RecordValue }

type FindingItem struct {
	ID         int64
	Title      string
	Severity   string // critical, high, medium, low, info
	Target     string
	SourceName string
}

func (f FindingItem) Type() ItemType           { return TypeFinding }
func (f FindingItem) ItemID() int64            { return f.ID }
func (f FindingItem) Source() string           { return f.SourceName }
func (f FindingItem) ScopeTarget() ScopeTarget { return ScopeTarget(f.Target) }
func (f FindingItem) Value() string            { return f.Title }

type FileItem struct {
	ID         int64
	Path       string // local filesystem path
	URL        string // origin URL (may be empty)
	SourceName string
}

func (f FileItem) Type() ItemType           { return TypeFile }
func (f FileItem) ItemID() int64            { return f.ID }
func (f FileItem) Source() string           { return f.SourceName }
func (f FileItem) ScopeTarget() ScopeTarget { return ScopeTarget(f.URL) }
func (f FileItem) Value() string            { return f.Path }
