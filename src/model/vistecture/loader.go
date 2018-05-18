package vistecture

import (
	"log"

	vistectureCore "github.com/AOEpeople/vistecture/model/core"
)

// loadProject loads the json file from a project folder
func LoadProject(path string) *vistectureCore.Project {
	project, err := vistectureCore.CreateProject(path)

	if err != nil {
		log.Fatal("Project JSON is not valid:", err)
	}

	err = project.Validate()

	if err != nil {
		log.Fatal("Validation Errors:", err)
	}

	return project
}
