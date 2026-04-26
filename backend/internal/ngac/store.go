package ngac

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-document-platform/internal/models"
)

// Store handles PostgreSQL persistence for the NGAC graph
type Store struct {
	db    *pgxpool.Pool
	graph *Graph
}

func NewStore(db *pgxpool.Pool, graph *Graph) *Store {
	return &Store{db: db, graph: graph}
}

// InitSchema creates the NGAC tables
func (s *Store) InitSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS ngac_nodes (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		node_type  TEXT NOT NULL CHECK (node_type IN ('U','UA','O','OA','PC')),
		properties JSONB DEFAULT '{}',
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS ngac_assignments (
		id        TEXT PRIMARY KEY,
		child_id  TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		parent_id TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		UNIQUE(child_id, parent_id)
	);

	CREATE TABLE IF NOT EXISTS ngac_associations (
		id         TEXT PRIMARY KEY,
		ua_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		oa_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		operations TEXT[] NOT NULL,
		UNIQUE(ua_id, oa_id)
	);

	CREATE TABLE IF NOT EXISTS users (
		id          TEXT PRIMARY KEY,
		username    TEXT UNIQUE NOT NULL,
		password    TEXT NOT NULL,
		ngac_node   TEXT REFERENCES ngac_nodes(id),
		created_at  TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS documents (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL,
		filename    TEXT NOT NULL,
		mime_type   TEXT,
		owner_id    TEXT REFERENCES users(id),
		ngac_node   TEXT REFERENCES ngac_nodes(id),
		created_at  TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_assignments_child ON ngac_assignments(child_id);
	CREATE INDEX IF NOT EXISTS idx_assignments_parent ON ngac_assignments(parent_id);
	CREATE INDEX IF NOT EXISTS idx_associations_ua ON ngac_associations(ua_id);
	CREATE INDEX IF NOT EXISTS idx_associations_oa ON ngac_associations(oa_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON ngac_nodes(node_type);
	CREATE INDEX IF NOT EXISTS idx_nodes_name ON ngac_nodes(name);
	`
	_, err := s.db.Exec(ctx, schema)
	return err
}

// LoadGraph loads the entire NGAC graph from PostgreSQL into memory
func (s *Store) LoadGraph(ctx context.Context) error {
	// Load nodes
	rows, err := s.db.Query(ctx, "SELECT id, name, node_type, properties, created_at FROM ngac_nodes")
	if err != nil {
		return fmt.Errorf("loading nodes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var n models.NGACNode
		var props map[string]string
		if err := rows.Scan(&n.ID, &n.Name, &n.NodeType, &props, &n.CreatedAt); err != nil {
			return fmt.Errorf("scanning node: %w", err)
		}
		n.Properties = props
		s.graph.AddNode(&n)
	}

	// Load assignments
	rows, err = s.db.Query(ctx, "SELECT id, child_id, parent_id FROM ngac_assignments")
	if err != nil {
		return fmt.Errorf("loading assignments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a models.Assignment
		if err := rows.Scan(&a.ID, &a.ChildID, &a.ParentID); err != nil {
			return fmt.Errorf("scanning assignment: %w", err)
		}
		if err := s.graph.AddAssignment(&a); err != nil {
			// Log but don't fail — data might have integrity issues
			fmt.Printf("Warning: skipping assignment %s: %v\n", a.ID, err)
		}
	}

	// Load associations
	rows, err = s.db.Query(ctx, "SELECT id, ua_id, oa_id, operations FROM ngac_associations")
	if err != nil {
		return fmt.Errorf("loading associations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a models.Association
		if err := rows.Scan(&a.ID, &a.UAID, &a.OAID, &a.Operations); err != nil {
			return fmt.Errorf("scanning association: %w", err)
		}
		if err := s.graph.AddAssociation(&a); err != nil {
			fmt.Printf("Warning: skipping association %s: %v\n", a.ID, err)
		}
	}

	return nil
}

// CreateNode creates a node in both DB and memory
func (s *Store) CreateNode(ctx context.Context, name, nodeType string, properties map[string]string) (*models.NGACNode, error) {
	if !models.IsValidNodeType(nodeType) {
		return nil, fmt.Errorf("invalid node type: %s", nodeType)
	}

	node := &models.NGACNode{
		ID:         uuid.New().String(),
		Name:       name,
		NodeType:   nodeType,
		Properties: properties,
		CreatedAt:  time.Now(),
	}

	if properties == nil {
		properties = map[string]string{}
	}

	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $2, $3, $4)",
		node.ID, node.Name, node.NodeType, properties)
	if err != nil {
		return nil, fmt.Errorf("inserting node: %w", err)
	}

	s.graph.AddNode(node)
	return node, nil
}

// CreateAssignment creates an assignment edge in both DB and memory
func (s *Store) CreateAssignment(ctx context.Context, childID, parentID string) (*models.Assignment, error) {
	a := &models.Assignment{
		ID:       uuid.New().String(),
		ChildID:  childID,
		ParentID: parentID,
	}

	// Validate in graph first (before DB write)
	if err := s.graph.AddAssignment(a); err != nil {
		return nil, err
	}

	// Remove from graph — we'll re-add after successful DB write
	s.graph.RemoveAssignment(childID, parentID)

	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3) ON CONFLICT (child_id, parent_id) DO NOTHING",
		a.ID, a.ChildID, a.ParentID)
	if err != nil {
		return nil, fmt.Errorf("inserting assignment: %w", err)
	}

	// Re-add to graph after successful DB write
	if err := s.graph.AddAssignment(a); err != nil {
		return nil, err
	}

	return a, nil
}

// RemoveAssignment removes an assignment from both DB and memory
func (s *Store) RemoveAssignment(ctx context.Context, childID, parentID string) error {
	_, err := s.db.Exec(ctx,
		"DELETE FROM ngac_assignments WHERE child_id = $1 AND parent_id = $2",
		childID, parentID)
	if err != nil {
		return err
	}

	s.graph.RemoveAssignment(childID, parentID)
	return nil
}

// CreateAssociation creates an association in both DB and memory
func (s *Store) CreateAssociation(ctx context.Context, uaID, oaID string, operations []string) (*models.Association, error) {
	a := &models.Association{
		ID:         uuid.New().String(),
		UAID:       uaID,
		OAID:       oaID,
		Operations: operations,
	}

	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES ($1, $2, $3, $4) ON CONFLICT (ua_id, oa_id) DO UPDATE SET operations = $4",
		a.ID, a.UAID, a.OAID, operations)
	if err != nil {
		return nil, fmt.Errorf("inserting association: %w", err)
	}

	if err := s.graph.AddAssociation(a); err != nil {
		return nil, err
	}

	return a, nil
}

// RemoveAssociationByUAOA removes association between UA and OA
func (s *Store) RemoveAssociationByUAOA(ctx context.Context, uaID, oaID string) error {
	// Find the association ID in graph
	assocs := s.graph.GetAssociationsFromUA(uaID)
	for _, a := range assocs {
		if a.OAID == oaID {
			s.graph.RemoveAssociationByID(a.ID)
			break
		}
	}

	_, err := s.db.Exec(ctx, "DELETE FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", uaID, oaID)
	return err
}

// DeleteNode removes a node and all its edges from both DB and memory
func (s *Store) DeleteNode(ctx context.Context, nodeID string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", nodeID)
	if err != nil {
		return err
	}
	s.graph.RemoveNode(nodeID)
	return nil
}

// FindNodeByName finds a node by name and type
func (s *Store) FindNodeByName(name, nodeType string) *models.NGACNode {
	return s.graph.FindNodeByName(name, nodeType)
}

// GetNodesByType returns nodes of a given type
func (s *Store) GetNodesByType(nodeType string) []*models.NGACNode {
	return s.graph.GetNodesByType(nodeType)
}

// CreateUser creates a user in the DB
func (s *Store) CreateUser(ctx context.Context, id, username, password, ngacNodeID string) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO users (id, username, password, ngac_node) VALUES ($1, $2, $3, $4)",
		id, username, password, ngacNodeID)
	return err
}

// GetUserByUsername retrieves a user by username
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password, ngac_node, created_at FROM users WHERE username = $1",
		username).Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// GetUserByID retrieves a user by ID
func (s *Store) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password, ngac_node, created_at FROM users WHERE id = $1",
		userID).Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// ListUsers lists all users with their department info
func (s *Store) ListUsers(ctx context.Context) ([]models.UserInfo, error) {
	rows, err := s.db.Query(ctx, "SELECT id, username, ngac_node FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.UserInfo
	for rows.Next() {
		var u models.UserInfo
		if err := rows.Scan(&u.ID, &u.Username, &u.NGACNodeID); err != nil {
			return nil, err
		}

		// Get department and company from graph
		parents := s.graph.GetParents(u.NGACNodeID)
		for _, p := range parents {
			if p.NodeType == models.NodeTypeUserAttribute && p.Name != "PublicUsers" {
				u.Department = p.Name
				// Get company (PC) from this UA
				grandparents := s.graph.GetParents(p.ID)
				for _, gp := range grandparents {
					if gp.NodeType == models.NodeTypePolicyClass && gp.Name != "PC_Global" {
						u.Company = gp.Name
					}
				}
			}
		}
		users = append(users, u)
	}
	return users, nil
}

// CreateDocument creates a document record in the DB
func (s *Store) CreateDocument(ctx context.Context, doc *models.Document) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO documents (id, title, filename, mime_type, owner_id, ngac_node) VALUES ($1, $2, $3, $4, $5, $6)",
		doc.ID, doc.Title, doc.Filename, doc.MimeType, doc.OwnerID, doc.NGACNodeID)
	return err
}

// GetDocument retrieves a document by ID
func (s *Store) GetDocument(ctx context.Context, docID string) (*models.Document, error) {
	var d models.Document
	err := s.db.QueryRow(ctx,
		"SELECT d.id, d.title, d.filename, d.mime_type, d.owner_id, COALESCE(u.username,''), d.ngac_node, d.created_at FROM documents d LEFT JOIN users u ON d.owner_id = u.id WHERE d.id = $1",
		docID).Scan(&d.ID, &d.Title, &d.Filename, &d.MimeType, &d.OwnerID, &d.OwnerName, &d.NGACNodeID, &d.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Determine status from graph
	d.Status = s.getDocumentStatus(d.NGACNodeID)
	d.IsPublic = s.isDocumentPublic(d.NGACNodeID)

	return &d, nil
}

// ListDocuments lists all documents
func (s *Store) ListDocuments(ctx context.Context) ([]models.Document, error) {
	rows, err := s.db.Query(ctx,
		"SELECT d.id, d.title, d.filename, d.mime_type, d.owner_id, COALESCE(u.username,''), d.ngac_node, d.created_at FROM documents d LEFT JOIN users u ON d.owner_id = u.id ORDER BY d.created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.Title, &d.Filename, &d.MimeType, &d.OwnerID, &d.OwnerName, &d.NGACNodeID, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Status = s.getDocumentStatus(d.NGACNodeID)
		d.IsPublic = s.isDocumentPublic(d.NGACNodeID)
		docs = append(docs, d)
	}
	return docs, nil
}

// DeleteDocument removes a document from DB
func (s *Store) DeleteDocument(ctx context.Context, docID string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM documents WHERE id = $1", docID)
	return err
}

// HasSeedData checks if seed data exists
func (s *Store) HasSeedData(ctx context.Context) bool {
	var count int
	err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM ngac_nodes WHERE node_type = 'PC'").Scan(&count)
	return err == nil && count > 0
}

// Helper: determine document status from NGAC graph
func (s *Store) getDocumentStatus(ngacNodeID string) string {
	parents := s.graph.GetParents(ngacNodeID)
	for _, p := range parents {
		if p.Name == "ApprovedDocs" {
			return "approved"
		}
	}
	// Check ancestors too (OA hierarchy)
	ancestors := s.graph.GetAncestors(ngacNodeID)
	for _, a := range ancestors {
		if a.Name == "ApprovedDocs" {
			return "approved"
		}
	}
	return "draft"
}

// Helper: check if document is public
func (s *Store) isDocumentPublic(ngacNodeID string) bool {
	parents := s.graph.GetParents(ngacNodeID)
	for _, p := range parents {
		if p.Name == "PublicDocs" {
			return true
		}
	}
	return false
}

// GetDocumentShareOAName generates the share OA name for a document-to-UA share
func GetDocumentShareOAName(docNodeID, targetUAID string) string {
	shortDoc := docNodeID
	if len(shortDoc) > 8 {
		shortDoc = shortDoc[:8]
	}
	shortUA := targetUAID
	if len(shortUA) > 8 {
		shortUA = shortUA[:8]
	}
	return fmt.Sprintf("Share_%s_to_%s", shortDoc, shortUA)
}

// FindShareOA finds the share OA for a document-to-UA share
func (s *Store) FindShareOA(docNodeID, targetUAID string) *models.NGACNode {
	name := GetDocumentShareOAName(docNodeID, targetUAID)
	return s.graph.FindNodeByName(name, models.NodeTypeObjectAttr)
}

// ListSharesForDocument returns all active shares for a document
func (s *Store) ListSharesForDocument(docNodeID string) []models.ShareInfo {
	var shares []models.ShareInfo

	// Find all OA parents of this document that start with "Share_"
	parents := s.graph.GetParents(docNodeID)
	for _, p := range parents {
		if p.NodeType == models.NodeTypeObjectAttr && strings.HasPrefix(p.Name, "Share_") {
			// Find associations pointing to this OA
			for _, assoc := range s.graph.Associations {
				if assoc.OAID == p.ID {
					uaNode := s.graph.GetNode(assoc.UAID)
					uaName := ""
					if uaNode != nil {
						uaName = uaNode.Name
					}
					shares = append(shares, models.ShareInfo{
						ID:           assoc.ID,
						TargetUAID:   assoc.UAID,
						TargetUAName: uaName,
						Operations:   assoc.Operations,
						ShareOAID:    p.ID,
					})
				}
			}
		}
	}

	return shares
}
