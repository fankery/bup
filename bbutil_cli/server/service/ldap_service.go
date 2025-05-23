package service

import (
	"bbutil_cli/common"
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/spf13/viper"
)

type LdapService struct{}

// 尝试绑定ldap服务
func ldapConn() *ldap.Conn {
	//尝试绑定ldap服务器
	common.Logger.Info("开始绑定ldap服务")
	ldapUrl := fmt.Sprintf("ldap://%s:%d", viper.GetString("ldap.host"), viper.GetInt("ldap.port"))
	bindDn := viper.GetString("ldap.bindDn")
	bindPassword := viper.GetString("ldap.passwd")
	conn, err := ldap.DialURL(ldapUrl)
	if err != nil {
		common.Logger.Error("ldap连接错误", err)
		return nil
	}
	// 绑定到ldap服务
	if err = conn.Bind(bindDn, bindPassword); err != nil {
		common.Logger.Error("ldap绑定错误", err)
		return nil
	}
	common.Logger.Info("ldap服务绑定成功")
	return conn

}

func (l *LdapService) LoginAuth(username, password string) bool {
	// 构建LDAP搜索过滤器
	searchFilter := fmt.Sprintf("(uid=%s)", username)

	// 执行LDAP搜索操作以查找用户
	searchRequest := ldap.NewSearchRequest(
		"dc=waiqin365,dc=com", ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		searchFilter, []string{"cn"}, nil,
	)
	conn := ldapConn()
	defer func() {
		conn.Close()
		common.Logger.Info("关闭ladp连接资源")
	}()
	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		// 处理搜索错误
		common.Logger.Error(err)
		return false
	}
	if len(searchResult.Entries) != 1 {
		// 用户不存在或者多个用户匹配
		common.Logger.Error("User not found or multiple users match")
		return false
	}
	userDN := searchResult.Entries[0].DN
	// 尝试绑定用户提供的密码
	err = conn.Bind(userDN, password)
	if err != nil {
		// 身份验证失败
		common.Logger.Error("Authentication failed")
		return false
	}
	// 身份验证成功
	common.Logger.Debug("Authentication successful")
	return true
}

func (l *LdapService) getUserId(username string) string {
	searchFilter := fmt.Sprintf("(uid=%s)", username)
	searchRequest := ldap.NewSearchRequest(
		"ou=nanjing,ou=emplyees,dc=waiqin365,dc=com",
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		searchFilter,
		[]string{"loginShell"}, // 返回的属性列表，这里只包含
		nil,
	)
	conn := ldapConn()
	defer func() {
		conn.Close()
		common.Logger.Info("关闭ladp连接资源")
	}()
	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		common.Logger.Error("ldap search err,", err)
		return ""
	}
	if len(searchResult.Entries) == 0 {
		common.Logger.Error("user not found: ", username)
		return ""
	}
	userId := searchResult.Entries[0].GetAttributeValue("loginShell")
	return userId
}
