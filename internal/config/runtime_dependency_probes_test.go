package config

import (
	"testing"
)

func TestCompileRuntimeDependencyProbesPhpFpmSocket(t *testing.T) {
	t.Parallel()
	specs, err := compileRuntimeDependencyProbes([]RuntimeDependencyProbe{
		{Type: "php_fpm", Socket: "/run/php/php8.3-fpm.sock"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Dependency != "php_fpm" || specs[0].SocketPath != "/run/php/php8.3-fpm.sock" {
		t.Fatalf("%+v", specs)
	}
}

func TestCompileRuntimeDependencyProbesRejectsPhpFpmTcp(t *testing.T) {
	t.Parallel()
	_, err := compileRuntimeDependencyProbes([]RuntimeDependencyProbe{
		{Type: "php_fpm", Socket: "/run/php/php8.3-fpm.sock", TCP: "127.0.0.1:9000"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileRuntimeDependencyProbesRedisTCP(t *testing.T) {
	t.Parallel()
	specs, err := compileRuntimeDependencyProbes([]RuntimeDependencyProbe{
		{Type: "redis", TCP: "127.0.0.1:6379"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].TCPAddr != "127.0.0.1:6379" {
		t.Fatalf("%+v", specs)
	}
}
