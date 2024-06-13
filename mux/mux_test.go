package mux

import (
	"net/http"
	"testing"

	"github.com/komuw/ong/config"
)

func TestNew(t *testing.T) {
	routes := func() []Route {
		return []Route{
			NewRoute("/home", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			NewRoute("/home/", MethodAll, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
		}
	}

	t.Run("conflict detected", func(t *testing.T) {
		rtz := []Route{}
		rtz = append(rtz, tarpitRoutes()...)
		rtz = append(rtz, routes()...)

		// attest.Panics(t, func() {
		_ = New(config.DevOpts(nil, "secretKey12@34String"), nil, rtz...)
		// })
	})
}

func tarpitRoutes() []Route {
	tarpitHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	routes := []Route{}

	for _, uri := range []string{
		// CMS

		"/images/",
		"/joomla/",
		"/libraries/joomla/",
		"/administrator/",
		"/components/",
		"/templates/",
		"/includes/",
		"/modules/",
		"/plugins/",
		"/drupal/",
		"/Drupal.php",

		// OTHERS
		"/echo.php",
		"/composer.php",
		"/uploader.php",
		"/shell.php",
		"/freenode-proxy-checker.txt",
		"/cmd.php",
		"/muhstiks.php",
		"/muhstik.php",
		"/jmx-console",
		"/status.php",
		"/TP/",
		"/HNAP1/",
		"/manager/",
		"/program/",
		"/shopdb/",
		"/programs/",
		"/jenkins/",
		"/w00tw00t.at.blackhats.romanian.anti-sec:)",
		"/judge.php",
		"/muieblackcat",
		"/.env",
		"/log",
		"/configs",
		"/config",
		"/cfg",
		"/gs",
		"/gsProvision",
		"/overrides",
		"/polycom",
		"/spa.xml",
		"/yealink",
		"/help.php",
		"/java.php",
		"/_query.php",
		"/test.php",
		"/db_cts.php",
		"/db_pma.php",
		"/logon.php",
		"/help-e.php",
		"/license.php",
		"/log.php",
		"/hell.php",
		"/pmd_online.php",
		"/x.php",
		"/htdocs.php",
		"/b.php",
		"/desktop.ini.php",
		"/z.php",
		"/lala.php",
		"/lala-dpr.php",
		"/wpc.php",
		"/wpo.php",
		"/t6nv.php",
		"/text.php",
		"/muhstik2.php",
		"/muhstik-dpr.php",
		"/lol.php",
		"/cmv.php",
		"/cmdd.php",
		"/knal.php",
		"/appserv.php",
		"/d7.php",
		"/rxr.php",
		"/1x.php",
		"/home.php",
		"/undx.php",
		"/spider.php",
		"/payload.php",
		"/composers.php",
		"/izom.php",
		"/hue2.php",
		"/new_license.php",
		"/up.php",
		// "/pmd/",
		// "/PMA/",
		// "/PMA2/",
		// "/pmamy/",
		// "/pmamy2/",
		// "/dbadmin/",
		// "/tools/",
		// "/phpma/",
		// "/php-my-admin/",
		// "/websql/",
		// "/dbadmin/",
		// "/xmlrpc.php",
		// "/user/",
		// "/vuln.htm",
		// "/webconfig.txt.php",
	} {
		uri := uri
		routes = append(
			routes,
			NewRoute(
				uri,
				MethodAll,
				tarpitHandler,
			),
		)

	}

	return routes
}
