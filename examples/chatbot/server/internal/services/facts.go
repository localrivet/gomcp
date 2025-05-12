package services

import (
	"fmt"
)

// FactsService provides operations for retrieving facts
type FactsService struct {
	facts map[string]string
}

// NewFactsService creates a new facts service with predefined facts
func NewFactsService() *FactsService {
	return &FactsService{
		facts: map[string]string{
			"earth":      "Earth is the third planet from the Sun and the only astronomical object known to harbor life.",
			"mars":       "Mars is the fourth planet from the Sun and the second-smallest planet in the Solar System, only being larger than Mercury.",
			"jupiter":    "Jupiter is the largest planet in the Solar System. It is the fifth planet from the Sun.",
			"python":     "Python is a high-level, general-purpose programming language. Its design philosophy emphasizes code readability.",
			"go":         "Go is a statically typed, compiled programming language designed at Google by Robert Griesemer, Rob Pike, and Ken Thompson.",
			"javascript": "JavaScript is a high-level, often just-in-time compiled language that conforms to the ECMAScript specification.",
			"coffee":     "Coffee is a brewed drink prepared from roasted coffee beans, the seeds of berries from certain Coffea species.",
			"tea":        "Tea is an aromatic beverage prepared by pouring hot or boiling water over cured or fresh leaves of Camellia sinensis.",
		},
	}
}

// GetFactAbout retrieves a fact about a given topic
func (s *FactsService) GetFactAbout(topic string) string {
	// Check if we have a fact for the provided topic
	if fact, ok := s.facts[topic]; ok {
		return fact
	}

	// Return a default response for unknown topics
	return fmt.Sprintf("I don't have specific facts about '%s'. Please try another topic or ask a more specific question.", topic)
}
