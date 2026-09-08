package dmmsave

import (
	"sdmm/internal/dmapi/dmenv"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"

	"github.com/rs/zerolog/log"
)

func Save(dme *dmenv.Dme, dmm *dmmap.Dmm, cfg Config) bool {
	return SaveV(dme, dmm, dmm.Path.Absolute, cfg)
}

func SaveV(dme *dmenv.Dme, dmm *dmmap.Dmm, path string, cfg Config) bool {
	log.Printf("save started [%s]...", path)

	sp, err := makeSaveProcess(cfg, dme, dmm, path)
	if err != nil {
		log.Print("unable to start save process")
		util.ShowErrorDialog("Unable to start save process")
		return false
	}

	if cfg.SanitizeVariables {
		sp.sanitizeVariables()
	}

	sp.handleReusedKeys()
	if err = sp.handleLocationsWithoutKeys(); err != nil {
		log.Print("unable to handle locations without keys:", err)
		util.ShowErrorDialog("Unable to save the map: " + err.Error())
		return false
	}
	if err = sp.output.Save(); err != nil {
		log.Print("unable to write map:", err)
		util.ShowErrorDialog("Unable to save the map: " + err.Error())
		return false
	}

	log.Print("save finished")
	return true
}
