// Package processor holds the govips/libvips image processing pipeline.
//
// The actual pipeline — libvips configuration, the worker semaphore, and the
// per-target preset definitions — is implemented in HRA-163. This file only
// declares the package so the module layout is in place.
package processor
