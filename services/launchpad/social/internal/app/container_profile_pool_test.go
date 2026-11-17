package app

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestContainerProfilePoolSearchPath(t *testing.T) {
	for _, environment := range []string{"production", "development", "test"} {
		for _, profile := range []string{"container", "platform", "owned_local", ""} {
			for _, tc := range []struct {
				name string
				path string
				want string
			}{
				{"default", "", "chat,public"},
				{"supplied", `"$user",public`, `chat,"$user",public`},
				{"chat supplied", "chat,public", "chat,chat,public"},
			} {
				t.Run(environment+"/"+profile+"/"+tc.name, func(t *testing.T) {
					given, err := url.Parse("postgresql://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable&application_name=sociald&connect_timeout=7&statement_timeout=12000&options=-c%20lock_timeout%3D2000")
					if err != nil {
						t.Fatal(err)
					}
					query := given.Query()
					if tc.path != "" {
						query.Set("search_path", tc.path)
					}
					given.RawQuery = query.Encode()
					cfg := config.Config{NodeEnv: environment, RuntimeProfile: profile, DatabaseURL: given.String()}
					dsn, err := runtimePoolDSN(cfg)
					if err != nil {
						t.Fatal(err)
					}
					actual, err := url.Parse(dsn)
					if err != nil {
						t.Fatal(err)
					}
					query.Set("search_path", tc.want)
					if !reflect.DeepEqual(actual.Query(), query) {
						t.Fatalf("pool parameters = %v; want %v", actual.Query(), query)
					}
					actual.RawQuery = given.RawQuery
					if !reflect.DeepEqual(actual, given) {
						t.Fatal("pool URL changed outside its search path")
					}
					poolConfig, err := pgxpool.ParseConfig(dsn)
					if err != nil {
						t.Fatal(err)
					}
					if got := poolConfig.ConnConfig.RuntimeParams["search_path"]; got != tc.want {
						t.Fatalf("pool search path = %q; want %q", got, tc.want)
					}
				})
			}
		}
	}
}

func TestContainerProfilePoolRejectsInvalidDSN(t *testing.T) {
	for _, profile := range []string{"container", "platform", "owned_local"} {
		t.Run(profile, func(t *testing.T) {
			dsn, err := runtimePoolDSN(config.Config{NodeEnv: "production", RuntimeProfile: profile, DatabaseURL: "postgres://%"})
			if err == nil || dsn != "" {
				t.Fatalf("invalid pool URL = %q, %v", dsn, err)
			}
		})
	}
}
