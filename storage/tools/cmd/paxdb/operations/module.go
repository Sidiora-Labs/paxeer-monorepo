package operations

import "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"

func selectedModules(module string, db *memiavl.DB) []string {
	if module != "" {
		return []string{module}
	}
	modules := make([]string, 0, len(db.Trees()))
	for _, tree := range db.Trees() {
		modules = append(modules, tree.Name)
	}
	return modules
}
