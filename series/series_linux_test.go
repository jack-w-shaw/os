// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package series_test

import (
	"io/ioutil"
	"path/filepath"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/os/v2/series"
)

type linuxVersionSuite struct {
	testing.CleanupSuite
}

var futureReleaseFileContents = `NAME="Ubuntu"
VERSION="99.04 LTS, Star Trek"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu spock (99.04 LTS)"
VERSION_ID="99.04"
`

var distroInfoContents = `version,codename,series,created,release,eol,eol-server
12.04 LTS,Precise Pangolin,precise,2011-10-13,2012-04-26,2017-04-26
99.04,Star Trek,spock,2364-04-25,2364-10-17,2365-07-17
`

var _ = gc.Suite(&linuxVersionSuite{})

func (s *linuxVersionSuite) SetUpTest(c *gc.C) {
	s.CleanupSuite.SetUpTest(c)

	cleanup := series.SetSeriesVersions(make(map[string]string))
	s.AddCleanup(func(*gc.C) { cleanup() })
}

func (s *linuxVersionSuite) TestOSVersion(c *gc.C) {
	// Set up fake /etc/os-release file from the future.
	d := c.MkDir()
	release := filepath.Join(d, "future-release")
	s.PatchValue(series.OSReleaseFile, release)
	err := ioutil.WriteFile(release, []byte(futureReleaseFileContents), 0666)
	c.Assert(err, jc.ErrorIsNil)

	// Set up fake /usr/share/distro-info/ubuntu.csv, also from the future.
	distroInfo := filepath.Join(d, "ubuntu.csv")
	err = ioutil.WriteFile(distroInfo, []byte(distroInfoContents), 0644)
	c.Assert(err, jc.ErrorIsNil)
	s.PatchValue(series.UbuntuDistroInfoPath, distroInfo)

	// Ensure the future series can be read even though Juju doesn't
	// know about it.
	version, err := series.ReadSeries()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(version, gc.Equals, "spock")

	// Ensure that we identify that the poly-filled os releases from distro-info
	// don't change supported values.
	series := series.UbuntuSupportedSeries()

	// Precise isn't poly-filled and isn't supported.
	precise, ok := series["precise"]
	c.Assert(ok, jc.IsTrue)
	c.Assert(precise.CreatedByLocalDistroInfo, jc.IsFalse)
	c.Assert(precise.Supported, jc.IsFalse)

	// Bionic isn't poly-filled and isn't supported.
	bionic, ok := series["bionic"]
	c.Assert(ok, jc.IsTrue)
	c.Assert(bionic.CreatedByLocalDistroInfo, jc.IsFalse)
	c.Assert(bionic.Supported, jc.IsFalse)

	// Spock is poly-filled and isn't supported.
	spock, ok := series["spock"]
	c.Assert(ok, jc.IsTrue)
	c.Assert(spock.CreatedByLocalDistroInfo, jc.IsTrue)
	c.Assert(spock.Supported, jc.IsFalse)
}

func (s *linuxVersionSuite) TestUseFastLXC(c *gc.C) {
	for i, test := range []struct {
		message        string
		releaseContent string
		expected       string
	}{{
		message: "missing release file",
	}, {
		message:        "OS release file is missing ID",
		releaseContent: "some junk\nand more junk",
	}, {
		message: "precise release",
		releaseContent: `
NAME="Ubuntu"
VERSION="12.04 LTS, Precise"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 12.04.3 LTS"
VERSION_ID="12.04"
`,
		expected: "12.04",
	}, {
		message: "trusty release",
		releaseContent: `
NAME="Ubuntu"
VERSION="14.04.1 LTS, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 14.04.1 LTS"
VERSION_ID="14.04"
`,
		expected: "14.04",
	}, {
		message: "minimal trusty release",
		releaseContent: `
ID=ubuntu
VERSION_ID="14.04"
`,
		expected: "14.04",
	}, {
		message: "minimal unstable unicorn",
		releaseContent: `
ID=ubuntu
VERSION_ID="14.10"
`,
		expected: "14.10",
	}, {
		message: "minimal jaunty",
		releaseContent: `
ID=ubuntu
VERSION_ID="9.10"
`,
		expected: "9.10",
	}} {
		c.Logf("%v: %v", i, test.message)
		filename := filepath.Join(c.MkDir(), "os-release")
		s.PatchValue(series.OSReleaseFile, filename)
		if test.releaseContent != "" {
			err := ioutil.WriteFile(filename, []byte(test.releaseContent+"\n"), 0644)
			c.Assert(err, jc.ErrorIsNil)
		}
		value := series.ReleaseVersion()
		c.Assert(value, gc.Equals, test.expected)
	}
}

type readSeriesSuite struct {
	testing.CleanupSuite
}

var _ = gc.Suite(&readSeriesSuite{})

func (s *readSeriesSuite) SetUpTest(c *gc.C) {
	s.CleanupSuite.SetUpTest(c)

	cleanup := series.SetSeriesVersions(make(map[string]string))
	s.AddCleanup(func(*gc.C) { cleanup() })
}

var readSeriesTests = []struct {
	contents string
	series   string
	err      string
}{{
	`NAME="Ubuntu"
VERSION="12.04.5 LTS, Precise Pangolin"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu precise (12.04.5 LTS)"
VERSION_ID="12.04"
`,
	"precise",
	"",
}, {
	`NAME="Ubuntu"
ID=ubuntu
VERSION_ID= "12.04" `,
	"precise",
	"",
}, {
	`NAME='Ubuntu'
ID='ubuntu'
VERSION_ID='12.04'
`,
	"precise",
	"",
}, {
	`NAME="CentOS Linux"
ID="centos"
VERSION_ID="7"
`,
	"centos7",
	"",
}, {
	`NAME="openSUSE Leap"
ID=opensuse
VERSION_ID="42.2"
`,
	"opensuseleap",
	"",
}, {
	`NAME="Ubuntu"
VERSION="14.04.1 LTS, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 14.04.1 LTS"
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"
`,
	"trusty",
	"",
}, {
	`NAME="Arch Linux"
ID=arch
PRETTY_NAME="Arch Linux"
ANSI_COLOR="0;36"
HOME_URL="https://www.archlinux.org/"
SUPPORT_URL="https://bbs.archlinux.org/"
BUG_REPORT_URL="https://bugs.archlinux.org/"
`,
	"genericlinux",
	"",
}, {
	`NAME=Fedora
VERSION="24 (Twenty Four)"
ID=fedora
VERSION_ID=24
PRETTY_NAME="Fedora 24 (Twenty Four)"
CPE_NAME="cpe:/o:fedoraproject:fedora:24"
HOME_URL="https://fedoraproject.org/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"
`,
	"genericlinux",
	"",
}, {
	`NAME="SuSE Linux"
ID="SuSE"
VERSION_ID="12"
`,
	"genericlinux",
	"",
}, {

	"",
	"unknown",
	"OS release file is missing ID",
}, {
	`NAME="CentOS Linux"
ID="centos"
`,
	"unknown",
	"could not determine series",
}, {
	`NAME=openSUSE
ID=opensuse
VERSION_ID="42.3"`,
	"opensuseleap",
	"",
},
}

func (s *readSeriesSuite) TestReadSeries(c *gc.C) {
	d := c.MkDir()
	f := filepath.Join(d, "foo")
	s.PatchValue(series.OSReleaseFile, f)
	for i, t := range readSeriesTests {
		c.Logf("test %d", i)
		err := ioutil.WriteFile(f, []byte(t.contents), 0666)
		c.Assert(err, jc.ErrorIsNil)
		series, err := series.ReadSeries()
		if t.err == "" {
			c.Assert(err, jc.ErrorIsNil)
		} else {
			c.Assert(err, gc.ErrorMatches, t.err)
		}

		c.Assert(series, gc.Equals, t.series)
	}
}
