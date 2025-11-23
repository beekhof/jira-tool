package config

// AddRecentAssignee adds a user to the recent assignees list (max 6 unique)
func (c *Config) AddRecentAssignee(userIdentifier string) {
	c.RecentAssignees = addToRecentList(c.RecentAssignees, userIdentifier, 6)
}

// AddRecentSprint adds a sprint to the recent sprints list (max 6 unique)
func (c *Config) AddRecentSprint(sprintName string) {
	c.RecentSprints = addToRecentList(c.RecentSprints, sprintName, 6)
}

// AddRecentRelease adds a release to the recent releases list (max 6 unique)
func (c *Config) AddRecentRelease(releaseName string) {
	c.RecentReleases = addToRecentList(c.RecentReleases, releaseName, 6)
}

// addToRecentList adds an item to a recent list, keeping only the last N unique items
// If the item already exists, it's moved to the end (most recent)
func addToRecentList(list []string, item string, maxSize int) []string {
	// Remove the item if it already exists
	result := []string{}
	for _, existing := range list {
		if existing != item {
			result = append(result, existing)
		}
	}
	
	// Add the item to the end
	result = append(result, item)
	
	// Keep only the last maxSize items
	if len(result) > maxSize {
		result = result[len(result)-maxSize:]
	}
	
	return result
}

