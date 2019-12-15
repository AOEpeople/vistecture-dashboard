package vistecture

import (
	"log"
	"path"

	vistectureCore "github.com/AOEpeople/vistecture/v2/model/core"
	"github.com/AOEpeople/vistecture/v2/application"
)

// loadProject loads the json file from a project folder
func LoadProject(projectConfigFile string) *vistectureCore.Project {
	log.Printf("Loading vistecture definition from %v", projectConfigFile)
	loader := application.ProjectLoader{}
	definitions, err := loader.LoadProjectConfig(projectConfigFile)
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	completeProject, err := loader.LoadProject(definitions, path.Dir(projectConfigFile), "")
	if err != nil {
		log.Fatal("Project JSON is not valid:", err)
		panic(err)
	}
	log.Printf("Loaded %v apps for project %v", len(completeProject.Applications),definitions.ProjectName)
	return completeProject
}
