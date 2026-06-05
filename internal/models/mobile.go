package models

import "time"

// ──────────────────────────────────────────────
// Sync
// ──────────────────────────────────────────────

type SyncAction struct {
	LocalID   string         `json:"local_id"`
	Type      string         `json:"type"`
	Entity    string         `json:"entity"`
	EntityID  int            `json:"entity_id"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type SyncPushRequest struct {
	TechnicianID int          `json:"technician_id"`
	DeviceID     string       `json:"device_id"`
	DeviceName   string       `json:"device_name,omitempty"`
	Platform     string       `json:"platform,omitempty"`
	LastSyncAt   *time.Time   `json:"last_sync_at,omitempty"`
	Actions      []SyncAction `json:"actions"`
}

type SyncActionResult struct {
	LocalID  string `json:"local_id"`
	Status   string `json:"status"`
	ServerID *int   `json:"server_id,omitempty"`
	Message  string `json:"message"`
}

type SyncPushResponse struct {
	SyncID     string             `json:"sync_id"`
	Status     string             `json:"status"`
	ServerTime time.Time          `json:"server_time"`
	Results    []SyncActionResult `json:"results"`
}

type SyncLog struct {
	ID           int
	TechnicianID int
	DeviceID     string
	LocalID      string
	ActionType   string
	Entity       string
	EntityID     *int
	Payload      map[string]any
	Status       string
	ErrorMessage string
	ServerID     *int
	CreatedAt    time.Time
}

// ──────────────────────────────────────────────
// Dashboard
// ──────────────────────────────────────────────

type DashboardStats struct {
	TechnicianID      int        `json:"technician_id"`
	AssignedCount     int        `json:"assigned_count"`
	AcceptedCount     int        `json:"accepted_count"`
	InProgressCount   int        `json:"in_progress_count"`
	UrgentCount       int        `json:"urgent_count"`
	ResolvedToday     int        `json:"resolved_today_count"`
	PendingSyncCount  int        `json:"pending_sync_count"`
	NextWorkOrder     any        `json:"next_work_order"`
	ServerTime        time.Time  `json:"server_time"`
}

// ──────────────────────────────────────────────
// Work Order (mobile view)
// ──────────────────────────────────────────────

type MobileLampadaire struct {
	ID        *int     `json:"id"`
	Reference string   `json:"reference"`
	Zone      string   `json:"zone"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Etat      string   `json:"etat"`
	Intensite int      `json:"intensite"`
	Puissance *float64 `json:"puissance"`
}

type MobileLCU struct {
	ID        *int   `json:"id"`
	Reference string `json:"reference"`
	IPAddress string `json:"ip_address"`
	Zone      string `json:"zone,omitempty"`
}

type MobileAlert struct {
	ID       *int   `json:"id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type MobileTelemetry struct {
	Temperature *float64   `json:"temperature"`
	Luminosite  *float64   `json:"luminosite"`
	Puissance   *float64   `json:"puissance"`
	Courant     *float64   `json:"courant"`
	Tension     *float64   `json:"tension"`
	CreatedAt   *time.Time `json:"created_at"`
}

type MobileWorkOrder struct {
	ID             int               `json:"id"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Status         string            `json:"status"`
	Priority       string            `json:"priority"`
	Zone           string            `json:"zone,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	AcceptedAt     *time.Time        `json:"accepted_at,omitempty"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty"`
	DueDate        *time.Time        `json:"due_date,omitempty"`
	TechnicianID   *int              `json:"technician_id,omitempty"`
	AssignedToName string            `json:"assigned_to_name,omitempty"`
	ResolutionNote string            `json:"resolution_note,omitempty"`
	Lampadaire     *MobileLampadaire `json:"lampadaire,omitempty"`
	LCU            *MobileLCU        `json:"lcu,omitempty"`
	Alert          *MobileAlert      `json:"alert,omitempty"`
	Telemetry      *MobileTelemetry  `json:"telemetry,omitempty"`
	Logs           []WorkOrderLog    `json:"logs,omitempty"`
}

type WorkOrderLog struct {
	ID        int        `json:"id"`
	UserName  string     `json:"user_name"`
	Action    string     `json:"action"`
	Note      string     `json:"note,omitempty"`
	OldStatus string     `json:"old_status,omitempty"`
	NewStatus string     `json:"new_status,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ──────────────────────────────────────────────
// Diagnostic
// ──────────────────────────────────────────────

type DiagnosticAlert struct {
	ID        int       `json:"id"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type DiagnosticResult struct {
	Lampadaire struct {
		ID                  int      `json:"id"`
		Reference           string   `json:"reference"`
		Zone                string   `json:"zone"`
		Etat                string   `json:"etat"`
		Intensite           int      `json:"intensite"`
		Puissance           *float64 `json:"puissance"`
		CommissioningStatus string   `json:"commissioning_status"`
		LastSeenAt          *string  `json:"last_seen_at"`
	} `json:"lampadaire"`
	LCU           *MobileLCU       `json:"lcu"`
	Telemetry     *MobileTelemetry `json:"telemetry"`
	OpenAlerts    []DiagnosticAlert `json:"open_alerts"`
	HasActiveAlerts bool           `json:"has_active_alerts"`
}

// ──────────────────────────────────────────────
// Map
// ──────────────────────────────────────────────

type MapLampadaire struct {
	ID               int      `json:"id"`
	Reference        string   `json:"reference"`
	Zone             string   `json:"zone"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	Etat             string   `json:"etat"`
	Intensite        int      `json:"intensite"`
	Puissance        *float64 `json:"puissance"`
	LCUID            *int     `json:"lcu_id"`
	HasCriticalAlert bool     `json:"has_critical_alert"`
}

type MapLCU struct {
	ID        int      `json:"id"`
	Reference string   `json:"reference"`
	Name      string   `json:"name"`
	IPAddress string   `json:"ip_address"`
	Zone      string   `json:"zone"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Status    string   `json:"status"`
}

type MapConnection struct {
	LCUID       int `json:"lcu_id"`
	LampadaireID int `json:"lampadaire_id"`
}

type TechnicianContext struct {
	TechnicianID        int             `json:"technician_id"`
	Position            *GeoPosition    `json:"position,omitempty"`
	AssignedLampadaires []MapLampadaire `json:"assigned_lampadaires"`
	NearbyLampadaires   []MapLampadaire `json:"nearby_lampadaires"`
	LCUs                []MapLCU        `json:"lcus"`
	Connections         []MapConnection `json:"connections"`
	WorkOrders          []MobileWorkOrder `json:"work_orders"`
	MissingLocation     []MapLampadaire `json:"missing_location"`
}

type GeoPosition struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ──────────────────────────────────────────────
// Device
// ──────────────────────────────────────────────

type MobileDevice struct {
	ID           int        `json:"id"`
	TechnicianID int        `json:"technician_id"`
	DeviceID     string     `json:"device_id"`
	DeviceName   string     `json:"device_name"`
	Platform     string     `json:"platform"`
	LastSyncAt   *time.Time `json:"last_sync_at"`
	CreatedAt    time.Time  `json:"created_at"`
}
