package desktop

import (
	"context"
	"log/slog"
	"os"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type desktopProfile string

const (
	DesktopProfileEnvVar                = "PNEUMA_DESKTOP_PROFILE"
	desktopProfileProd   desktopProfile = "prod"
	desktopProfileDev    desktopProfile = "dev"
)

// resolveDesktopProfile determines the desktop profile to use based on environment variables and build type.
func resolveDesktopProfile(ctx context.Context) desktopProfile {
	if profile, ok := desktopProfileFromEnv(); ok {
		return profile
	}

	buildType := strings.ToLower(strings.TrimSpace(wailsRuntime.Environment(ctx).BuildType))
	if buildType == "dev" {
		return desktopProfileDev
	}

	return desktopProfileProd
}

// desktopProfileFromEnv checks the environment variable for a desktop profile override and validates it.
func desktopProfileFromEnv() (desktopProfile, bool) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(DesktopProfileEnvVar)))
	switch value {
	case "":
		return "", false
	case string(desktopProfileDev):
		return desktopProfileDev, true
	case string(desktopProfileProd):
		return desktopProfileProd, true
	default:
		slog.Warn("invalid desktop profile override, ignoring", "env", DesktopProfileEnvVar, "value", value)
		return "", false
	}
}

// desktopAppDir returns the appropriate application directory name based on the desktop profile.
func desktopAppDir(profile desktopProfile) string {
	if profile == desktopProfileDev {
		return DesktopDevAppDirName
	}
	return DesktopProdAppDirName
}

// thumbnailsTempDir returns the appropriate temporary directory name for thumbnails based on the desktop profile.
func thumbnailsTempDir(profile desktopProfile) string {
	if profile == desktopProfileDev {
		return ThumbnailsTempDirDev
	}
	return ThumbnailsTempDirProd
}
