// Package snapshot serialises schema models into deterministic JSON documents.
//
// Snapshots are the stable intermediate representation that future drift,
// diff, rename, and destructive-change workflows will compare before producing
// migration SQL.
package snapshot
