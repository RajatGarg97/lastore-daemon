/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import "dbus/org/freedesktop/accounts"
import "internal/system"
import "fixme/pkg_recommend"
import "pkg.deepin.io/lib/dbus"

// QueryLangByUID query Language from org.freedesktop.Accounts
func QueryLangByUID(uid int64) (string, error) {
	ac, err := accounts.NewAccounts("org.freedesktop.Accounts", "/org/freedesktop/Accounts")
	if err != nil {
		return "", err
	}
	defer accounts.DestroyAccounts(ac)
	upath, err := ac.FindUserById(uid)
	if err != nil {
		return "", err
	}

	u, err := accounts.NewUser("org.freedesktop.Accounts", upath)
	if err != nil {
		return "", err
	}
	defer accounts.DestroyUser(u)
	lang := u.Language.Get()
	if lang == "" {
		return "", system.NotFoundError
	}
	return lang, nil
}

func QueryEnhancedLocalePackages(checker func(string) bool, lang string, pkgs ...string) []string {
	set := make(map[string]struct{})
	for _, pkg := range pkgs {
		for _, localePkg := range pkg_recommend.GetEnhancedLocalePackages(lang, pkg) {
			set[localePkg] = struct{}{}
		}
	}

	var r []string
	for pkg := range set {
		if checker(pkg) {
			r = append(r, pkg)
		}
	}
	return r
}

// Don't directly use this API. It will be migration to com.deepin.Accounts
func (m *Manager) RecordLocaleInfo(msg dbus.DMessage, locale string) error {
	uid := msg.GetSenderUID()
	if locale == "" {
		return system.NotFoundError
	}
	m.cachedLocale[uint64(uid)] = locale
	return nil
}
