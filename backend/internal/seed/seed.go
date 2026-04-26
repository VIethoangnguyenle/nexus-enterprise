package seed

import (
	"context"
	"fmt"

	"ngac-document-platform/internal/auth"
	"ngac-document-platform/internal/models"
	"ngac-document-platform/internal/ngac"

	"github.com/google/uuid"
)

// SeedData creates the initial NGAC graph, companies, departments, users, and sample documents
func SeedData(ctx context.Context, store *ngac.Store) error {
	if store.HasSeedData(ctx) {
		fmt.Println("Seed data already exists, skipping...")
		return nil
	}

	fmt.Println("Seeding NGAC graph...")

	// === Policy Classes ===
	pcAcme, _ := store.CreateNode(ctx, "PC_Acme", models.NodeTypePolicyClass, nil)
	pcBeta, _ := store.CreateNode(ctx, "PC_Beta", models.NodeTypePolicyClass, nil)
	pcGlobal, _ := store.CreateNode(ctx, "PC_Global", models.NodeTypePolicyClass, nil)

	// === User Attributes (Departments) ===
	acmeFinance, _ := store.CreateNode(ctx, "Acme_Finance", models.NodeTypeUserAttribute, map[string]string{"company": "PC_Acme"})
	acmeHR, _ := store.CreateNode(ctx, "Acme_HR", models.NodeTypeUserAttribute, map[string]string{"company": "PC_Acme"})
	acmeEng, _ := store.CreateNode(ctx, "Acme_Engineering", models.NodeTypeUserAttribute, map[string]string{"company": "PC_Acme"})
	betaEng, _ := store.CreateNode(ctx, "Beta_Engineering", models.NodeTypeUserAttribute, map[string]string{"company": "PC_Beta"})
	betaMkt, _ := store.CreateNode(ctx, "Beta_Marketing", models.NodeTypeUserAttribute, map[string]string{"company": "PC_Beta"})
	publicUsers, _ := store.CreateNode(ctx, "PublicUsers", models.NodeTypeUserAttribute, map[string]string{})

	// Assign UAs to PCs
	store.CreateAssignment(ctx, acmeFinance.ID, pcAcme.ID)
	store.CreateAssignment(ctx, acmeHR.ID, pcAcme.ID)
	store.CreateAssignment(ctx, acmeEng.ID, pcAcme.ID)
	store.CreateAssignment(ctx, betaEng.ID, pcBeta.ID)
	store.CreateAssignment(ctx, betaMkt.ID, pcBeta.ID)
	store.CreateAssignment(ctx, publicUsers.ID, pcGlobal.ID)

	// === Object Attributes ===
	acmeFinDocs, _ := store.CreateNode(ctx, "Acme_Finance_Docs", models.NodeTypeObjectAttr, nil)
	acmeHRDocs, _ := store.CreateNode(ctx, "Acme_HR_Docs", models.NodeTypeObjectAttr, nil)
	acmeEngDocs, _ := store.CreateNode(ctx, "Acme_Engineering_Docs", models.NodeTypeObjectAttr, nil)
	betaEngDocs, _ := store.CreateNode(ctx, "Beta_Engineering_Docs", models.NodeTypeObjectAttr, nil)
	betaMktDocs, _ := store.CreateNode(ctx, "Beta_Marketing_Docs", models.NodeTypeObjectAttr, nil)

	// Workflow OAs
	draftDocs, _ := store.CreateNode(ctx, "DraftDocs", models.NodeTypeObjectAttr, nil)
	approvedDocs, _ := store.CreateNode(ctx, "ApprovedDocs", models.NodeTypeObjectAttr, nil)

	// Sharing/Public OAs
	sharedDocs, _ := store.CreateNode(ctx, "SharedDocs", models.NodeTypeObjectAttr, nil)
	publicDocs, _ := store.CreateNode(ctx, "PublicDocs", models.NodeTypeObjectAttr, nil)

	// Assign OAs to PCs
	store.CreateAssignment(ctx, acmeFinDocs.ID, pcAcme.ID)
	store.CreateAssignment(ctx, acmeHRDocs.ID, pcAcme.ID)
	store.CreateAssignment(ctx, acmeEngDocs.ID, pcAcme.ID)
	store.CreateAssignment(ctx, betaEngDocs.ID, pcBeta.ID)
	store.CreateAssignment(ctx, betaMktDocs.ID, pcBeta.ID)
	store.CreateAssignment(ctx, draftDocs.ID, pcGlobal.ID)
	store.CreateAssignment(ctx, approvedDocs.ID, pcGlobal.ID)
	store.CreateAssignment(ctx, sharedDocs.ID, pcGlobal.ID)
	store.CreateAssignment(ctx, publicDocs.ID, pcGlobal.ID)

	// === Associations (permissions) ===
	allOps := []string{"read", "write", "upload", "approve", "share"}
	readOnly := []string{"read"}

	store.CreateAssociation(ctx, acmeFinance.ID, acmeFinDocs.ID, allOps)
	store.CreateAssociation(ctx, acmeHR.ID, acmeHRDocs.ID, allOps)
	store.CreateAssociation(ctx, acmeEng.ID, acmeEngDocs.ID, allOps)
	store.CreateAssociation(ctx, betaEng.ID, betaEngDocs.ID, allOps)
	store.CreateAssociation(ctx, betaMkt.ID, betaMktDocs.ID, allOps)

	// All departments can approve drafts
	store.CreateAssociation(ctx, acmeFinance.ID, draftDocs.ID, []string{"read", "approve"})
	store.CreateAssociation(ctx, acmeHR.ID, draftDocs.ID, []string{"read", "approve"})
	store.CreateAssociation(ctx, acmeEng.ID, draftDocs.ID, []string{"read", "approve"})
	store.CreateAssociation(ctx, betaEng.ID, draftDocs.ID, []string{"read", "approve"})
	store.CreateAssociation(ctx, betaMkt.ID, draftDocs.ID, []string{"read", "approve"})

	// All departments can read approved docs
	store.CreateAssociation(ctx, acmeFinance.ID, approvedDocs.ID, readOnly)
	store.CreateAssociation(ctx, acmeHR.ID, approvedDocs.ID, readOnly)
	store.CreateAssociation(ctx, acmeEng.ID, approvedDocs.ID, readOnly)
	store.CreateAssociation(ctx, betaEng.ID, approvedDocs.ID, readOnly)
	store.CreateAssociation(ctx, betaMkt.ID, approvedDocs.ID, readOnly)

	// PublicUsers can read PublicDocs
	store.CreateAssociation(ctx, publicUsers.ID, publicDocs.ID, readOnly)

	// === Users ===
	createSeedUser(ctx, store, "alice", "password123", acmeFinance, publicUsers)
	createSeedUser(ctx, store, "bob", "password123", acmeHR, publicUsers)
	createSeedUser(ctx, store, "charlie", "password123", betaEng, publicUsers)
	createSeedUser(ctx, store, "dave", "password123", betaMkt, publicUsers)

	// === Sample Documents ===
	// Acme invoice (draft)
	inv, _ := store.CreateNode(ctx, "Q4 Invoice Report", models.NodeTypeObject, map[string]string{"doc_id": "doc-invoice-1", "filename": "invoice.pdf"})
	store.CreateAssignment(ctx, inv.ID, acmeFinDocs.ID)
	store.CreateAssignment(ctx, inv.ID, draftDocs.ID)
	store.CreateDocument(ctx, &models.Document{ID: "doc-invoice-1", Title: "Q4 Invoice Report", Filename: "invoice.pdf", MimeType: "application/pdf", OwnerID: getUserID(ctx, store, "alice"), NGACNodeID: inv.ID})

	// Acme handbook (approved)
	hb, _ := store.CreateNode(ctx, "Employee Handbook", models.NodeTypeObject, map[string]string{"doc_id": "doc-handbook-1", "filename": "handbook.pdf"})
	store.CreateAssignment(ctx, hb.ID, acmeHRDocs.ID)
	store.CreateAssignment(ctx, hb.ID, approvedDocs.ID)
	store.CreateDocument(ctx, &models.Document{ID: "doc-handbook-1", Title: "Employee Handbook", Filename: "handbook.pdf", MimeType: "application/pdf", OwnerID: getUserID(ctx, store, "bob"), NGACNodeID: hb.ID})

	// Beta spec (approved + shared with Acme_Engineering)
	spec, _ := store.CreateNode(ctx, "API Spec v2", models.NodeTypeObject, map[string]string{"doc_id": "doc-spec-1", "filename": "spec-v2.pdf"})
	store.CreateAssignment(ctx, spec.ID, betaEngDocs.ID)
	store.CreateAssignment(ctx, spec.ID, approvedDocs.ID)
	store.CreateDocument(ctx, &models.Document{ID: "doc-spec-1", Title: "API Spec v2", Filename: "spec-v2.pdf", MimeType: "application/pdf", OwnerID: getUserID(ctx, store, "charlie"), NGACNodeID: spec.ID})

	// Share spec with Acme_Engineering
	shareOA, _ := store.CreateNode(ctx, ngac.GetDocumentShareOAName(spec.ID, acmeEng.ID), models.NodeTypeObjectAttr, map[string]string{"share_type": "cross-company"})
	store.CreateAssignment(ctx, shareOA.ID, sharedDocs.ID)
	store.CreateAssignment(ctx, spec.ID, shareOA.ID)
	store.CreateAssociation(ctx, acmeEng.ID, shareOA.ID, readOnly)

	// Public document
	pub, _ := store.CreateNode(ctx, "Company Newsletter", models.NodeTypeObject, map[string]string{"doc_id": "doc-newsletter-1", "filename": "newsletter.pdf"})
	store.CreateAssignment(ctx, pub.ID, acmeHRDocs.ID)
	store.CreateAssignment(ctx, pub.ID, approvedDocs.ID)
	store.CreateAssignment(ctx, pub.ID, publicDocs.ID)
	store.CreateDocument(ctx, &models.Document{ID: "doc-newsletter-1", Title: "Company Newsletter", Filename: "newsletter.pdf", MimeType: "application/pdf", OwnerID: getUserID(ctx, store, "bob"), NGACNodeID: pub.ID})

	fmt.Println("Seed data created successfully!")
	return nil
}

func createSeedUser(ctx context.Context, store *ngac.Store, username, password string, deptUA, publicUA *models.NGACNode) {
	hashedPw, _ := auth.HashPassword(password)
	userNode, _ := store.CreateNode(ctx, username, models.NodeTypeUser, map[string]string{
		"department": deptUA.Name,
	})
	store.CreateAssignment(ctx, userNode.ID, deptUA.ID)
	store.CreateAssignment(ctx, userNode.ID, publicUA.ID)

	userID := uuid.New().String()
	store.CreateUser(ctx, userID, username, hashedPw, userNode.ID)
}

func getUserID(ctx context.Context, store *ngac.Store, username string) string {
	user, _ := store.GetUserByUsername(ctx, username)
	if user != nil {
		return user.ID
	}
	return ""
}
