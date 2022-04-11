package venus

import (
	"context"
	"fmt"
	"github.com/drakkan/sftpgo/v2/dataprovider"
	"github.com/drakkan/sftpgo/v2/logger"
	"github.com/drakkan/sftpgo/v2/vfs"
	"github.com/sftpgo/sdk"
	"io"
	"path"
	"path/filepath"
	"time"
)

type local2Mix struct {
	*distribution
	osn      *dataprovider.User
	ossFs    vfs.Fs
	osFs     vfs.Fs
	backuped bool
}

func (l *local2Mix) checkFile(ctx context.Context) error {
	sourcePath := l.getSourcePath()
	if sourcePath == "" {
		return fmt.Errorf("文件路径为空！")
	}
	fs, err := l.osn.GetFilesystemForPath(sourcePath, "")
	if err != nil {
		return fmt.Errorf("发起用户vfs系统获取错误，请检查用户配置 %v", err)
	}
	sourceStat, err := fs.Stat(sourcePath)
	if err != nil || sourceStat.IsDir() {
		return fmt.Errorf("文件不存在，或是文件夹 %v", err)
	}
	return nil
}

func (l *local2Mix) backupFile(ctx context.Context) error {
	if l.rule.Backup == dataprovider.VenusRuleFileBackupNot {
		l.backuped = false
		return fmt.Errorf("用户不进行文件备份")
	}
	sourcePath := l.getSourcePath()
	backupPath := l.getBackupPath()

	fsSrc, err := l.osn.GetFilesystemForPath(sourcePath, "")
	if err != nil {
		return err
	}
	fsDst, err := l.osn.GetFilesystemForPath(backupPath, "")
	if err != nil {
		return err
	}
	err = l.copyFile(ctx, fsSrc, fsDst, sourcePath, backupPath)
	if err != nil {
		return err
	}
	l.log.backupSize = l.msg.FileSize
	l.backuped = true
	return nil
}

func (l *local2Mix) copyFile(ctx context.Context, fsSrc, fsDst vfs.Fs, src, dst string) error {
	if fsDst == fsSrc && l.osn.FsConfig.Provider == sdk.S3FilesystemProvider {
		if err := l.ossFs.Copy(src, dst); err != nil {
			return fmt.Errorf("备份文件出错 %v", err)
		}
		//没有报错，证明复制完成
		l.log.backupSize = l.msg.FileSize
	} else {
		var writer io.WriteCloser
		var reader io.ReadCloser

		srcFile, r, cancelSrc, err := fsSrc.Open(src, 0)
		if srcFile != nil {
			reader = srcFile
		} else {
			if err != nil {
				cancelSrc()
				return fmt.Errorf("无法打开文件 %#v 去读取: %+v", src, err)
			}
			reader = r
			defer cancelSrc()
		}
		defer reader.Close()

		dstFile, w, cancelDst, err := fsDst.Create(dst, 0)

		if dstFile != nil {
			writer = dstFile
		} else {
			if err != nil {
				cancelDst()
				return fmt.Errorf("无法创建文件 %#v: %+v", dst, err)
			}
			writer = w
			defer cancelSrc()
		}
		defer writer.Close()

		wSize, err := io.Copy(writer, reader)
		if err != nil {
			return err
		}
		logger.InfoToConsole("拷贝文件：%v 至：%v 大小: %v", src, dst, wSize)
	}
	return nil
}

func (l *local2Mix) distributionFile(ctx context.Context) error {
	sourcePath := l.getSourcePath()
	backupPath := l.getBackupPath()
	if len(l.rule.HSN) == 0 {
		return fmt.Errorf("落地路径为空，无需流转")
	}

	fsSrc, err := l.osn.GetFilesystemForPath(sourcePath, "")
	if err != nil {
		return err
	}

	l.log.hsn = make([]sn, len(l.rule.HSN))
	for i, hsn := range l.rule.HSN {
		//本地流转
		if hsn.LocalUser != "" {
			hsnLocal, err := hsn.GetLocalUser()
			if err != nil {
				l.log.hsn[i].err = err.Error()
				continue
			}
			targetPath := l.getLocalTargetPath(hsnLocal, hsn.RelativePath)

			//如果是创建软连接，证明两者是在一个文件系统里面
			if l.rule.Symlink == dataprovider.VenusRuleFileSymlink && l.snDataProviderEqual(l.osn, hsnLocal) {
				var comFs vfs.Fs
				switch l.osn.FsConfig.Provider {
				case sdk.LocalFilesystemProvider:
					comFs = l.osFs
				case sdk.S3FilesystemProvider:
					comFs = l.ossFs
				default:
					//其他类型不支持
					continue
				}
				if l.rule.Backup == dataprovider.VenusRuleFileBackup && l.backuped {
					//后续考虑加上本地os存储的
					err := l.symlinkFile(ctx, backupPath, targetPath, comFs)
					if err != nil {
						l.log.hsn[i].err = err.Error()
					}
					continue
				}
				if !l.backuped {
					//后续考虑加上本地os存储的
					err := l.symlinkFile(ctx, sourcePath, targetPath, comFs)
					if err != nil {
						l.log.hsn[i].err = err.Error()
					}
					continue
				}
			} else if l.rule.Symlink == dataprovider.VenusRuleFileSymlinkNot {
				fsDst, err := hsnLocal.GetFilesystemForPath(targetPath, "")
				if err != nil {
					l.log.hsn[i].err = err.Error()
					continue
				}
				err = l.copyFile(ctx, fsSrc, fsDst, backupPath, targetPath)
				if err != nil {
					l.log.hsn[i].err = err.Error()
					continue
				}
			}
		} else {
			//TODO 本地到远端的逻辑
		}

	}
	return nil
}

func (l *local2Mix) deleteFile(ctx context.Context) error {
	sourcePath := l.getSourcePath()
	fsSrc, err := l.osn.GetFilesystemForPath(sourcePath, "")
	if err != nil {
		l.log.deleteErr = err.Error()
		return err
	}
	if err := fsSrc.Remove(sourcePath, false); err != nil {
		l.log.deleteErr = err.Error()
		return err
	}
	return nil
}

func (l *local2Mix) symlinkFile(ctx context.Context, source, target string, commonFs vfs.Fs) error {
	if err := commonFs.Symlink(source, target); err != nil {
		logger.ErrorToConsole("创建连接文件出错 %v", err)
		return err
	}
	return nil
}
func (l *local2Mix) run(ctx context.Context) {
	err := l.setupOSN(ctx)
	if err != nil {
		l.log.err = err.Error()
		return
	}
	l.setupCommonFs(ctx)
	//if err != nil {
	//	logger.Error(logSender, l.msg.SessionID, err.Error())
	//}
	//检查文件是否存在
	err = l.checkFile(ctx)
	if err != nil {
		l.log.err = err.Error()
		return
	}
	//备份文件
	err = l.backupFile(ctx)
	if err != nil {
		l.log.err = err.Error()
		l.log.backupErr = err.Error()
		return
	}
	//流转文件
	err = l.distributionFile(ctx)
	if err != nil {
		l.log.err = err.Error()
		return
	}
	//删除文件
	err = l.deleteFile(ctx)
	if err != nil {
		l.log.err = err.Error()
		return
	}
	return
}

//返回绝对路径
func (l *local2Mix) getSourcePath() string {
	return l.msg.Path
}

//本地流转存储之中的绝对路径
func (l *local2Mix) getLocalTargetPath(hsnLocal *dataprovider.User, relativePath string) string {
	//远端
	fileName := path.Base(l.msg.Path)
	fsConfig := &hsnLocal.FsConfig
	switch hsnLocal.FsConfig.Provider {
	case sdk.S3FilesystemProvider:
		return filepath.Join("/", fsConfig.S3Config.KeyPrefix, relativePath, fileName)
	case sdk.GCSFilesystemProvider:
		return filepath.Join("/", fsConfig.GCSConfig.KeyPrefix, relativePath, fileName)
	case sdk.AzureBlobFilesystemProvider:
		return filepath.Join("/", fsConfig.AzBlobConfig.KeyPrefix, relativePath, fileName)
	case sdk.CryptedFilesystemProvider:
		return filepath.Join("/", fileName)
	case sdk.SFTPFilesystemProvider:
		return filepath.Join("/", fsConfig.SFTPConfig.Prefix, relativePath, fileName)
	default:
		return filepath.Join(hsnLocal.HomeDir, relativePath, fileName)
	}
}

//返回在存储之中的绝对路径
func (l *local2Mix) getBackupPath() string {
	suffix := "BAK"
	fileName := path.Base(l.msg.Path)
	switch l.osn.FsConfig.Provider {
	case sdk.S3FilesystemProvider:
		return filepath.Join("/", l.osn.FsConfig.S3Config.KeyPrefix, suffix, fileName)
	case sdk.GCSFilesystemProvider:
		return filepath.Join("/", l.osn.FsConfig.GCSConfig.KeyPrefix, suffix, fileName)
	case sdk.AzureBlobFilesystemProvider:
		return filepath.Join("/", l.osn.FsConfig.AzBlobConfig.KeyPrefix, suffix, fileName)
	case sdk.CryptedFilesystemProvider:
		return filepath.Join("/", suffix, fileName)
	case sdk.SFTPFilesystemProvider:
		return filepath.Join("/", l.osn.FsConfig.SFTPConfig.Prefix, suffix, fileName)
	default:
		return filepath.Join(l.osn.HomeDir, suffix, fileName)
	}
}

func (l *local2Mix) setupOSN(ctx context.Context) error {
	osn, err := l.rule.OSN.GetLocalUser()
	if err != nil {
		l.log.osn = sn{
			username: l.msg.Username,
			filepath: "",
			provider: 0,
			addr:     "",
			datetime: time.Now().UnixMilli(),
			err:      err.Error(),
		}
		return err
	}
	l.log.osn = sn{
		username: l.msg.Username,
		filepath: l.msg.VirtualPath,
		provider: l.msg.FsProvider,
		addr:     venusConfig.Addr,
		datetime: time.Now().UnixMilli(),
		err:      "",
	}
	l.osn = osn
	return nil
}

func (l *local2Mix) setupCommonFs(ctx context.Context) error {
	if l.osn.FsConfig.Provider != sdk.S3FilesystemProvider {
		return fmt.Errorf("当前provider不支持 %v", l.osn.FsConfig.Provider.Name())
	}
	ossFs, err := getOssFsByS3Config(l.osn.FsConfig.S3Config)
	if err != nil {
		return err
	}
	l.ossFs = ossFs
	l.osFs = vfs.NewOsFs("", "", "")
	return nil
}

func (l *local2Mix) snDataProviderEqual(osnUser, hsnUser *dataprovider.User) bool {
	if osnUser == hsnUser && osnUser != nil {
		return true
	}
	if osnUser.FsConfig.Provider == hsnUser.FsConfig.Provider {
		//目前暂时不支持s3(oss)和本地以外的存储
		switch osnUser.FsConfig.Provider {
		case sdk.LocalFilesystemProvider:
			return true
		case sdk.S3FilesystemProvider:
			osnConfig := &osnUser.FsConfig.S3Config
			hsnConfig := &osnUser.FsConfig.S3Config
			return osnConfig.Bucket == hsnConfig.Bucket &&
				osnConfig.Endpoint == hsnConfig.Endpoint &&
				osnConfig.AccessKey == hsnConfig.AccessKey
		}
	}
	return false
}

func newLocal2Mix(d *distribution) VService {
	return &local2Mix{
		distribution: d,
		osn:          nil,
	}
}
