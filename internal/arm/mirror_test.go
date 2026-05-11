package arm

import "testing"

func TestApplyMirror(t *testing.T) {
	cases := []struct {
		name, url, mirror, want string
	}{
		{
			name:   "empty mirror passes through",
			url:    "https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi.tar.xz",
			mirror: "",
			want:   "https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi.tar.xz",
		},
		{
			name:   "non-arm url passes through",
			url:    "https://example.com/foo.tar.xz",
			mirror: "https://mirror.example.com",
			want:   "https://example.com/foo.tar.xz",
		},
		{
			name:   "https mirror prefix swap",
			url:    "https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi.tar.xz",
			mirror: "https://internal.example.com/arm",
			want:   "https://internal.example.com/arm/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi.tar.xz",
		},
		{
			name:   "trailing slash on mirror is stripped",
			url:    "https://developer.arm.com/foo/bar.tar.xz",
			mirror: "https://internal.example.com/arm/",
			want:   "https://internal.example.com/arm/foo/bar.tar.xz",
		},
		{
			name:   "file:// mirror works for local network share",
			url:    "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-linux.tar.bz2",
			mirror: "file:///mnt/share/arm-mirror",
			want:   "file:///mnt/share/arm-mirror/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-linux.tar.bz2",
		},
		{
			name:   "bare local path mirror",
			url:    "https://developer.arm.com/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-linux.tar.bz2",
			mirror: "/srv/arm-mirror",
			want:   "/srv/arm-mirror/-/media/files/downloads/gnu-rm/9-2019q4/gcc-arm-none-eabi-9-2019-q4-major-linux.tar.bz2",
		},
		{
			name:   "query string from mangled URL is stripped",
			url:    "https://developer.arm.com/-/media/files/downloads/gnu-rm/5_3-2016q1/gcc-arm-none-eabi-5_3-2016q1-20160330-linux.tar.bz2?revision=417e2623-c259-4a12-aacc-c87200b569c7",
			mirror: "https://internal.example.com/arm",
			want:   "https://internal.example.com/arm/-/media/files/downloads/gnu-rm/5_3-2016q1/gcc-arm-none-eabi-5_3-2016q1-20160330-linux.tar.bz2",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ApplyMirror(c.url, c.mirror)
			if got != c.want {
				t.Errorf("ApplyMirror(%q, %q)\n got: %q\nwant: %q", c.url, c.mirror, got, c.want)
			}
		})
	}
}
