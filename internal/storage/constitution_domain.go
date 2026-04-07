// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// ConstitutionLayer defines the scope at which a constitution applies.
type ConstitutionLayer string

// ConstitutionLayer values.
const (
	ConstitutionLayerUnspecified ConstitutionLayer = ""
	ConstitutionLayerUser        ConstitutionLayer = "user"
	ConstitutionLayerOrg         ConstitutionLayer = "org"
	ConstitutionLayerProject     ConstitutionLayer = "project"
	ConstitutionLayerDomain      ConstitutionLayer = "domain"
)

// Constitution represents the project's ground truth configuration.
// JSON tags are intentional — constitutions are loaded from YAML/JSON config files
// via encoding/json, unlike other domain types which use manual field mapping.
type Constitution struct {
	ID           string            `json:"id,omitempty"`
	Layer        ConstitutionLayer `json:"layer,omitempty"`
	Name         string            `json:"name,omitempty"`
	Version      int32             `json:"version,omitempty"`
	SourceURL    string            `json:"source_url,omitempty"`
	SourceHash   string            `json:"source_hash,omitempty"`
	Tech         *TechStack        `json:"tech,omitempty"`
	Principles   []Principle       `json:"principles,omitempty"`
	Process      *ProcessConfig    `json:"process,omitempty"`
	Constraints  []string          `json:"constraints,omitempty"`
	Antipatterns []Antipattern     `json:"antipatterns,omitempty"`
	References   []Reference       `json:"references,omitempty"`
	CreatedAt    time.Time         `json:"created_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}

// TechStack describes the technology stack for a project.
type TechStack struct {
	Languages      *Languages        `json:"languages,omitempty"`
	Frameworks     map[string]string `json:"frameworks,omitempty"`
	Infrastructure map[string]string `json:"infrastructure,omitempty"`
	APIStandards   map[string]string `json:"api_standards,omitempty"`
	Data           map[string]string `json:"data,omitempty"`
}

// Languages specifies which programming languages are permitted.
type Languages struct {
	Primary          string            `json:"primary,omitempty"`
	Allowed          []string          `json:"allowed,omitempty"`
	Forbidden        []string          `json:"forbidden,omitempty"`
	ForbiddenReasons map[string]string `json:"forbidden_reasons,omitempty"`
}

// Principle captures a guiding design or engineering principle.
type Principle struct {
	ID         string `json:"id,omitempty"`
	Statement  string `json:"statement,omitempty"`
	Rationale  string `json:"rationale,omitempty"`
	Exceptions string `json:"exceptions,omitempty"`
	Delete     bool   `json:"$delete,omitempty"`
}

// ProcessConfig describes the team's review and deployment processes.
type ProcessConfig struct {
	SpecReview     string                `json:"spec_review,omitempty"`
	SecurityReview *SecurityReviewConfig `json:"security_review,omitempty"`
	Deployment     *DeploymentConfig     `json:"deployment,omitempty"`
	Documentation  *DocumentationConfig  `json:"documentation,omitempty"`
}

// SecurityReviewConfig describes when security reviews are required.
type SecurityReviewConfig struct {
	When string `json:"when,omitempty"`
}

// DeploymentConfig describes the deployment strategy.
type DeploymentConfig struct {
	Strategy string `json:"strategy,omitempty"`
	Rollback string `json:"rollback,omitempty"`
}

// DocumentationConfig describes documentation requirements.
type DocumentationConfig struct {
	APIDocs string `json:"api_docs,omitempty"`
	Runbook string `json:"runbook,omitempty"`
}

// Antipattern describes a known bad practice to avoid.
type Antipattern struct {
	Pattern string `json:"pattern,omitempty"`
	Why     string `json:"why,omitempty"`
	Instead string `json:"instead,omitempty"`
	Delete  bool   `json:"$delete,omitempty"`
}

// Reference points to an external document related to the constitution.
type Reference struct {
	Type   string `json:"type,omitempty"`
	Path   string `json:"path,omitempty"`
	Delete bool   `json:"$delete,omitempty"`
}
