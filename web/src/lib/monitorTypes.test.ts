import { describe, it, expect } from "vitest";
import { MONITOR_TYPE_INFO, isValidTarget } from "./monitorTypes";
import { MONITOR_TYPES } from "@/lib/store";

describe("MONITOR_TYPE_INFO", () => {
  it("describes every check type the app offers", () => {
    for (const type of MONITOR_TYPES) {
      expect(MONITOR_TYPE_INFO[type]).toBeDefined();
      expect(MONITOR_TYPE_INFO[type].targetLabel).not.toBe("");
      expect(MONITOR_TYPE_INFO[type].placeholder).not.toBe("");
    }
  });

  it("uses a target that is valid for its own type as the placeholder", () => {
    for (const type of MONITOR_TYPES) {
      expect(isValidTarget(type, MONITOR_TYPE_INFO[type].placeholder)).toBe(true);
    }
  });
});

describe("isValidTarget", () => {
  it("rejects an empty target for every type", () => {
    for (const type of MONITOR_TYPES) {
      expect(isValidTarget(type, "")).toBe(false);
    }
  });

  it("accepts http and https URLs for http checks", () => {
    expect(isValidTarget("http", "https://example.com/health")).toBe(true);
    expect(isValidTarget("http", "http://example.com:8080")).toBe(true);
  });

  it("rejects non-http schemes and bare hosts for http checks", () => {
    expect(isValidTarget("http", "ftp://example.com")).toBe(false);
    expect(isValidTarget("http", "example.com")).toBe(false);
  });

  it("requires host:port for tcp checks", () => {
    expect(isValidTarget("tcp", "db.example.com:5432")).toBe(true);
    expect(isValidTarget("tcp", "[2001:db8::1]:5432")).toBe(true);
    expect(isValidTarget("tcp", "db.example.com")).toBe(false);
    expect(isValidTarget("tcp", "db.example.com:0")).toBe(false);
    expect(isValidTarget("tcp", "db.example.com:70000")).toBe(false);
  });

  it("takes a bare host for ping and dns checks", () => {
    expect(isValidTarget("ping", "192.168.1.1")).toBe(true);
    expect(isValidTarget("ping", "2001:db8::1")).toBe(true);
    expect(isValidTarget("dns", "example.com")).toBe(true);
    expect(isValidTarget("dns", "sub.example.com.")).toBe(true);
  });

  it("rejects an IP literal as a DNS target, since looking one up queries nothing", () => {
    expect(isValidTarget("dns", "1.1.1.1")).toBe(false);
    expect(isValidTarget("dns", "2001:db8::1")).toBe(false);
    // The same literal is the normal case for a ping monitor.
    expect(isValidTarget("ping", "1.1.1.1")).toBe(true);
  });

  it("accepts hostnames with underscores, which Docker's resolver serves", () => {
    expect(isValidTarget("tcp", "db_primary:5432")).toBe(true);
    expect(isValidTarget("ping", "db_primary")).toBe(true);
    expect(isValidTarget("dns", "db_primary.internal")).toBe(true);
  });

  it("rejects a host carrying a scheme, port or path", () => {
    expect(isValidTarget("ping", "https://example.com")).toBe(false);
    expect(isValidTarget("ping", "example.com:80")).toBe(false);
    expect(isValidTarget("dns", "example.com/zone")).toBe(false);
  });
});
