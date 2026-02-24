package decrypt

import (
	"encoding/hex"
	"os"
	"testing"
)

// TestValidateKey 测试已知密钥是否能解开指定的微信数据库
//
// 使用方式:
//
//	go test -v -run TestValidateKey ./internal/wechat/decrypt/ \
//	  -key="你的hex密钥" \
//	  -datadir="/path/to/wechat/data" \
//	  -platform="darwin" \
//	  -version=4
var (
	// TODO: 填入你的密钥（64位hex字符串，即32字节）
	flagKey      = "c98e7b3c3cd85d7fbcf581ded0dca08fc4ccbea8c162bb667584e6d20580db8f"
	flagDataDir  = os.ExpandEnv("$HOME/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/wxid_q1xdpghqkofm21_8639")
	flagPlatform = "darwin"
	flagVersion  = 4
)

func init() {
	// 通过环境变量传参，避免 flag 冲突
	if v := os.Getenv("WX_KEY"); v != "" {
		flagKey = v
	}
	if v := os.Getenv("WX_DATADIR"); v != "" {
		flagDataDir = v
	}
	if v := os.Getenv("WX_PLATFORM"); v != "" {
		flagPlatform = v
	}
	if v := os.Getenv("WX_VERSION"); v != "" {
		if v == "4" {
			flagVersion = 4
		}
	}
}

func TestValidateKey(t *testing.T) {
	if flagKey == "" || flagDataDir == "" {
		t.Skip("跳过: 请设置环境变量 WX_KEY 和 WX_DATADIR，例如:\n" +
			"  WX_KEY=aabbccdd... WX_DATADIR=/path/to/wechat/data go test -v -run TestValidateKey ./internal/wechat/decrypt/")
	}

	keyBytes, err := hex.DecodeString(flagKey)
	if err != nil {
		t.Fatalf("密钥 hex 解码失败: %v", err)
	}

	if len(keyBytes) != 32 {
		t.Fatalf("密钥长度错误: 期望 32 字节, 实际 %d 字节", len(keyBytes))
	}

	validator, err := NewValidator(flagPlatform, flagVersion, flagDataDir)
	if err != nil {
		t.Fatalf("创建 Validator 失败: %v", err)
	}

	dbFile := GetSimpleDBFile(flagPlatform, flagVersion)
	t.Logf("平台: %s, 版本: v%d", flagPlatform, flagVersion)
	t.Logf("数据库文件: %s", dbFile)
	t.Logf("数据目录: %s", flagDataDir)
	t.Logf("密钥: %s", flagKey)

	if validator.ValidateDerived(keyBytes) {
		t.Logf("✅ 验证成功! 密钥可以解开数据库")
	} else {
		t.Errorf("❌ 验证失败! 密钥无法解开数据库")
	}
}

func TestValidateImgKey(t *testing.T) {
	if flagKey == "" || flagDataDir == "" {
		t.Skip("跳过: 请设置环境变量 WX_KEY 和 WX_DATADIR，例如:\n" +
			"  WX_KEY=aabbccdd... WX_DATADIR=/path/to/wechat/data go test -v -run TestValidateImgKey ./internal/wechat/decrypt/")
	}

	if flagVersion != 4 {
		t.Skip("跳过: 图片密钥验证仅支持 v4")
	}

	keyBytes, err := hex.DecodeString(flagKey)
	if err != nil {
		t.Fatalf("密钥 hex 解码失败: %v", err)
	}

	if len(keyBytes) != 16 {
		t.Fatalf("图片密钥长度错误: 期望 16 字节, 实际 %d 字节", len(keyBytes))
	}

	validator, err := NewValidator(flagPlatform, flagVersion, flagDataDir)
	if err != nil {
		t.Fatalf("创建 Validator 失败: %v", err)
	}

	t.Logf("平台: %s, 版本: v%d", flagPlatform, flagVersion)
	t.Logf("数据目录: %s", flagDataDir)
	t.Logf("图片密钥: %s", flagKey)

	if validator.ValidateImgKey(keyBytes) {
		t.Logf("✅ 图片密钥验证成功!")
	} else {
		t.Errorf("❌ 图片密钥验证失败!")
	}
}

