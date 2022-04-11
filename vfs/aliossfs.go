package vfs

import (
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/drakkan/sftpgo/v2/metric"
	"github.com/drakkan/sftpgo/v2/plugin"
	"github.com/drakkan/sftpgo/v2/util"
	"github.com/eikenb/pipeat"
	"github.com/pkg/sftp"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AliOSSFs struct {
	connectionID string
	localTempDir string
	// if not empty this fs is mouted as virtual folder in the specified path
	mountPath      string
	config         *AliOSSFsConfig
	svc            *oss.Client
	ctxTimeout     time.Duration
	ctxLongTimeout time.Duration
}

func (fs *AliOSSFs) getFileNamesInPrefix(fsPrefix string) (map[string]bool, error) {
	return nil, nil
}

func (fs *AliOSSFs) Name() string {
	return fmt.Sprintf("AliOSSFs bucket %#v", fs.config.Bucket)
}

func (fs *AliOSSFs) ConnectionID() string {
	return fs.connectionID
}

func (fs *AliOSSFs) Stat(name string) (os.FileInfo, error) {
	var result *FileInfo
	if name == "/" || name == "." {
		err := fs.checkIfBucketExists()
		if err != nil {
			return result, err
		}
		return updateFileInfoModTime(fs.getStorageID(), name, NewFileInfo(name, true, 0, time.Now(), false))
	}
	if "/"+fs.config.KeyPrefix == name+"/" {
		return NewFileInfo(name, true, 0, time.Now(), false), nil
	}
	obj, err := fs.headObject(name)
	if err == nil {
		// a "dir" has a trailing "/" so we cannot have a directory here
		objSize, err := strconv.ParseInt(obj.Get("Content-Length"), 10, 64)
		if err != nil {
			return nil, err
		}
		objectModTime, err := time.Parse(time.RFC1123, obj.Get("Last-Modified"))
		if err != nil {
			return nil, err
		}
		return updateFileInfoModTime(fs.getStorageID(), name, NewFileInfo(name, false, objSize, objectModTime, false))
	}
	if !fs.IsNotExist(err) {
		return result, err
	}
	// now check if this is a prefix (virtual directory)
	hasContents, err := fs.hasContents(name)
	if err == nil && hasContents {
		return updateFileInfoModTime(fs.getStorageID(), name, NewFileInfo(name, true, 0, time.Now(), false))
	} else if err != nil {
		return nil, err
	}
	// the requested file may still be a directory as a zero bytes key
	// with a trailing forward slash (created using mkdir).
	// S3 doesn't return content type when listing objects, so we have
	// create "dirs" adding a trailing "/" to the key
	return fs.getStatForDir(name)
}

func (fs *AliOSSFs) Lstat(name string) (os.FileInfo, error) {
	return fs.Stat(name)
}

func (fs *AliOSSFs) Open(name string, offset int64) (File, *pipeat.PipeReaderAt, func(), error) {
	return nil, nil, nil, ErrVfsUnsupported
}

func (fs *AliOSSFs) Create(name string, flag int) (File, *PipeWriter, func(), error) {
	return nil, nil, nil, ErrVfsUnsupported
}

func (fs *AliOSSFs) Rename(source, target string) error {
	if source == target {
		return nil
	}
	fi, err := fs.Stat(source)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("不支持文件夹重命名")
	}
	if err := fs.Copy(source, target); err != nil {
		return err
	}
	//文件元信息
	//if plugin.Handler.HasMetadater() {
	//	if !fi.IsDir() {
	//		err = plugin.Handler.SetModificationTime(fs.getStorageID(), ensureAbsPath(target),
	//			util.GetTimeAsMsSinceEpoch(fi.ModTime()))
	//		if err != nil {
	//			fsLog(fs, logger.LevelWarn, "unable to preserve modification time after renaming %#v -> %#v: %v",
	//				source, target, err)
	//		}
	//	}
	//}
	return fs.Remove(source, fi.IsDir())
}

func (fs *AliOSSFs) Remove(name string, isDir bool) error {
	if isDir {
		hasContents, err := fs.hasContents(name)
		if err != nil {
			return err
		}
		if hasContents {
			return fmt.Errorf("cannot remove non empty directory: %#v", name)
		}
		if !strings.HasSuffix(name, "/") {
			name += "/"
		}
	}
	bucket, err := fs.svc.Bucket(fs.config.Bucket)
	if err != nil {
		return err
	}
	if isDir {
		// 列举所有包含指定前缀的文件并删除。
		marker := oss.Marker("")
		// 指定待删除的文件前缀，例如dir。
		prefix := oss.Prefix(name)
		count := 0
		for {
			lor, err := bucket.ListObjects(marker, prefix)
			if err != nil {
				return err
			}
			objects := []string{}
			for _, object := range lor.Objects {
				objects = append(objects, object.Key)
			}
			// 将oss.DeleteObjectsQuiet设置为true，表示不返回删除结果。
			delRes, err := bucket.DeleteObjects(objects, oss.DeleteObjectsQuiet(true))
			if err != nil {
				return err
			}
			if len(delRes.DeletedObjects) > 0 {
				return err
			}
			count += len(objects)
			prefix = oss.Prefix(lor.Prefix)
			marker = oss.Marker(lor.NextMarker)
			if !lor.IsTruncated {
				break
			}
		}
	}
	err = bucket.DeleteObject(name)
	//metric.S3DeleteObjectCompleted(err)
	//if plugin.Handler.HasMetadater() && err == nil && !isDir {
	//	if errMetadata := plugin.Handler.RemoveMetadata(fs.getStorageID(), ensureAbsPath(name)); errMetadata != nil {
	//		fsLog(fs, logger.LevelWarn, "unable to remove metadata for path %#v: %v", name, errMetadata)
	//	}
	//}
	return err
}

func (fs *AliOSSFs) Mkdir(name string) error {
	_, err := fs.Stat(name)
	if !fs.IsNotExist(err) {
		return err
	}
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}
	_, w, _, err := fs.Create(name, -1)
	if err != nil {
		return err
	}
	return w.Close()
}

func (fs *AliOSSFs) MkdirAll(name string, uid int, gid int) error {
	return nil
}

func (fs *AliOSSFs) Symlink(source, target string) error {
	if source == target {
		return nil
	}
	bucket, err := fs.svc.Bucket(fs.config.Bucket)
	if err != nil {
		return err
	}
	if source != "/" && source != "." {
		source = strings.TrimPrefix(source, "/")
	}
	if target != "/" && target != "." {
		target = strings.TrimPrefix(target, "/")
	}
	// 创建软链接。
	option := []oss.Option{
		// 指定创建软链接时是否覆盖同名Object。
		oss.ForbidOverWrite(true),
		// 指定Object的访问权限。此处指定为Private，表示私有访问权限。
	}
	err = bucket.PutSymlink(target, source, option...)
	return err
}

func (fs *AliOSSFs) Chown(name string, uid int, gid int) error {
	return ErrVfsUnsupported
}

func (fs *AliOSSFs) Chmod(name string, mode os.FileMode) error {
	return ErrVfsUnsupported
}

func (fs *AliOSSFs) Chtimes(name string, atime, mtime time.Time, isUploading bool) error {
	if !plugin.Handler.HasMetadater() {
		return ErrVfsUnsupported
	}
	if !isUploading {
		info, err := fs.Stat(name)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return ErrVfsUnsupported
		}
	}
	return plugin.Handler.SetModificationTime(fs.getStorageID(), ensureAbsPath(name),
		util.GetTimeAsMsSinceEpoch(mtime))
}

func (fs *AliOSSFs) Truncate(name string, size int64) error {
	return ErrVfsUnsupported
}

func (fs *AliOSSFs) ReadDir(dirname string) ([]os.FileInfo, error) {
	return nil, ErrVfsUnsupported
}

func (fs *AliOSSFs) Readlink(name string) (string, error) {
	return "", ErrVfsUnsupported
}

func (fs *AliOSSFs) IsUploadResumeSupported() bool {
	return false
}

func (fs *AliOSSFs) IsAtomicUploadSupported() bool {
	return false
}

func (fs *AliOSSFs) CheckRootPath(username string, uid int, gid int) bool {
	// we need a local directory for temporary files
	osFs := NewOsFs(fs.ConnectionID(), fs.localTempDir, "")
	return osFs.CheckRootPath(username, uid, gid)
}

func (fs *AliOSSFs) ResolvePath(virtualPath string) (string, error) {
	if fs.mountPath != "" {
		virtualPath = strings.TrimPrefix(virtualPath, fs.mountPath)
	}
	if !path.IsAbs(virtualPath) {
		virtualPath = path.Clean("/" + virtualPath)
	}
	return fs.Join("/", fs.config.KeyPrefix, virtualPath), nil
}

func (fs *AliOSSFs) IsNotExist(err error) bool {
	if err == nil {
		return false
	}
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == s3.ErrCodeNoSuchKey {
			return true
		}
		if aerr.Code() == s3.ErrCodeNoSuchBucket {
			return true
		}
	}
	if multierr, ok := err.(s3manager.MultiUploadFailure); ok {
		if multierr.Code() == s3.ErrCodeNoSuchKey {
			return true
		}
		if multierr.Code() == s3.ErrCodeNoSuchBucket {
			return true
		}
	}
	return strings.Contains(err.Error(), "404")
}

func (fs *AliOSSFs) IsPermission(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "403")
}

func (fs *AliOSSFs) IsNotSupported(err error) bool {
	if err == nil {
		return false
	}
	return err == ErrVfsUnsupported
}

func (fs *AliOSSFs) ScanRootDirContents() (int, int64, error) {
	return 0, 0, nil
}

func (fs *AliOSSFs) GetDirSize(dirname string) (int, int64, error) {
	return 0, 0, ErrVfsUnsupported
}

func (fs *AliOSSFs) GetAtomicUploadPath(name string) string {
	return ""
}

func (fs *AliOSSFs) GetRelativePath(name string) string {
	rel := path.Clean(name)
	if rel == "." {
		rel = ""
	}
	if !path.IsAbs(rel) {
		return "/" + rel
	}
	if fs.config.KeyPrefix != "" {
		if !strings.HasPrefix(rel, "/"+fs.config.KeyPrefix) {
			rel = "/"
		}
		rel = path.Clean("/" + strings.TrimPrefix(rel, "/"+fs.config.KeyPrefix))
	}
	if fs.mountPath != "" {
		rel = path.Join(fs.mountPath, rel)
	}
	return rel
}

func (fs *AliOSSFs) Walk(root string, walkFn filepath.WalkFunc) error {
	return nil
}

func (fs *AliOSSFs) Join(elem ...string) string {
	return path.Join(elem...)
}

func (fs *AliOSSFs) HasVirtualFolders() bool {
	return true
}

func (fs *AliOSSFs) GetMimeType(name string) (string, error) {
	obj, err := fs.headObject(name)
	if err != nil {
		return "", err
	}
	return obj.Get("Content-Type"), err
}

func (fs *AliOSSFs) GetAvailableDiskSize(dirName string) (*sftp.StatVFS, error) {
	return nil, ErrStorageSizeUnavailable
}

func (fs *AliOSSFs) CheckMetadata() error {
	return fsMetadataCheck(fs, fs.getStorageID(), fs.config.KeyPrefix)
}

func (fs *AliOSSFs) Close() error {
	return nil
}

func (fs *AliOSSFs) Copy(source, target string) error {
	if source == target {
		return nil
	}
	fi, err := fs.Stat(source)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("不支持文件夹拷贝")
	}
	bucket, err := fs.svc.Bucket(fs.config.Bucket)
	if err != nil {
		return err
	}
	options := []oss.Option{
		oss.MetadataDirective(oss.MetaReplace),
		//oss.Expires(expires),
		//oss.SetTagging(taggingInfo),
		// 指定复制源Object的对象标签到目标 Object。
		// oss.TaggingDirective(oss.TaggingCopy),
		// 指定创建目标Object时的访问权限ACL为私有。
		// oss.ObjectACL(oss.ACLPrivate),
		// 指定KMS托管的用户主密钥，该参数仅在x-oss-server-side-encryption为KMS时有效。
		//oss.ServerSideEncryptionKeyID("9468da86-3509-4f8d-a61e-6eab1eac****"),
		// 指定OSS创建目标Object时使用的服务器端加密算法。
		// oss.ServerSideEncryption("AES256"),
		// 指定复制源Object的元数据到目标Object。
		//oss.MetadataDirective(oss.MetaCopy),
		// 指定CopyObject操作时是否覆盖同名目标Object。此处设置为true，表示禁止覆盖同名Object。
		// oss.ForbidOverWrite(true),
		// 如果源Object的ETag值和您提供的ETag相等，则执行拷贝操作，并返回200 OK。
		//oss.CopySourceIfMatch("5B3C1A2E053D763E1B002CC607C5****"),
		// 如果源Object的ETag值和您提供的ETag不相等，则执行拷贝操作，并返回200 OK。
		//oss.CopySourceIfNoneMatch("5B3C1A2E053D763E1B002CC607C5****"),
		// 如果指定的时间早于文件实际修改时间，则正常拷贝文件，并返回200 OK。
		//oss.CopySourceIfModifiedSince(2021-12-09T07:01:56.000Z),
		// 如果指定的时间等于或者晚于文件实际修改时间，则正常拷贝文件，并返回200 OK。
		//oss.CopySourceIfUnmodifiedSince(2021-12-09T07:01:56.000Z),
		// 指定Object的存储类型。此处设置为Standard，表示标准存储类型。
		//oss.StorageClass("Standard"),
	}

	if source != "/" && source != "." {
		source = strings.TrimPrefix(source, "/")
	}
	if target != "/" && target != "." {
		target = strings.TrimPrefix(target, "/")
	}
	_, err = bucket.CopyObject(source, target, options...)
	//if plugin.Handler.HasMetadater() {
	//	if !fi.IsDir() {
	//		err = plugin.Handler.SetModificationTime(fs.getStorageID(), ensureAbsPath(target),
	//			util.GetTimeAsMsSinceEpoch(fi.ModTime()))
	//		if err != nil {
	//			fsLog(fs, logger.LevelWarn, "unable to preserve modification time after renaming %#v -> %#v: %v",
	//				source, target, err)
	//		}
	//	}
	//}
	return err
}

func (fs *AliOSSFs) checkIfBucketExists() error {
	_, err := fs.svc.IsBucketExist(fs.config.Bucket)
	if err != nil {
		return err
	}
	return nil
}

func (fs *AliOSSFs) getStorageID() string {
	if fs.config.Endpoint != "" {
		if !strings.HasSuffix(fs.config.Endpoint, "/") {
			return fmt.Sprintf("oss://%v/%v", fs.config.Endpoint, fs.config.Bucket)
		}
		return fmt.Sprintf("oss://%v%v", fs.config.Endpoint, fs.config.Bucket)
	}
	return fmt.Sprintf("oss://%v", fs.config.Bucket)
}

func (fs *AliOSSFs) headObject(name string) (*http.Header, error) {
	if name != "/" && name != "." {
		name = strings.TrimPrefix(name, "/")
	}
	bucket, err := fs.svc.Bucket(fs.config.Bucket)
	if err != nil {
		return nil, err
	}
	if name != "/" && name != "." {
		name = strings.TrimPrefix(name, "/")
	}
	obj, err := bucket.GetObjectDetailedMeta(name)
	return &obj, err
}

func (fs *AliOSSFs) hasContents(name string) (bool, error) {
	prefix := ""
	if name != "/" && name != "." {
		prefix = strings.TrimPrefix(name, "/")
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
	}
	bucket, err := fs.svc.Bucket(fs.config.Bucket)
	if err != nil {
		return false, err
	}
	maxResults := 2
	results, err := bucket.ListObjectsV2(oss.MaxKeys(maxResults), oss.Prefix(prefix))
	metric.S3ListObjectsCompleted(err)
	if err != nil {
		return false, err
	}
	// MinIO returns no contents while S3 returns 1 object
	// with the key equal to the prefix for empty directories
	for _, obj := range results.Objects {
		name, _ := fs.resolve(&obj.Key, prefix)
		if name == "" || name == "/" {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (fs *AliOSSFs) getStatForDir(name string) (os.FileInfo, error) {
	var result *FileInfo
	obj, err := fs.headObject(name + "/")
	if err != nil {
		return result, err
	}
	objSize, err := strconv.ParseInt(obj.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, err
	}
	objectModTime, err := time.Parse(time.RFC1123, obj.Get("Last-Modified"))
	if err != nil {
		return nil, err
	}
	return updateFileInfoModTime(fs.getStorageID(), name, NewFileInfo(name, true, objSize, objectModTime, false))
}

func (fs *AliOSSFs) resolve(name *string, prefix string) (string, bool) {
	result := strings.TrimPrefix(*name, prefix)
	isDir := strings.HasSuffix(result, "/")
	if isDir {
		result = strings.TrimSuffix(result, "/")
	}
	if strings.Contains(result, "/") {
		i := strings.Index(result, "/")
		isDir = true
		result = result[:i]
	}
	return result, isDir
}

func NewAliOSSFs(connectionID, localTempDir, mountPath string, config AliOSSFsConfig, isValidated bool) (Fs, error) {
	if localTempDir == "" {
		if tempPath != "" {
			localTempDir = tempPath
		} else {
			localTempDir = filepath.Clean(os.TempDir())
		}
	}
	fs := &AliOSSFs{
		connectionID:   connectionID,
		localTempDir:   localTempDir,
		mountPath:      mountPath,
		config:         &config,
		ctxTimeout:     30 * time.Second,
		ctxLongTimeout: 300 * time.Second,
	}
	var err error
	if isValidated {
		if err = fs.config.Validate(); err != nil {
			return fs, err
		}
	}
	//上传大小设置
	if fs.config.UploadPartSize == 0 {
		fs.config.UploadPartSize = s3manager.DefaultUploadPartSize
	} else {
		fs.config.UploadPartSize *= 1024 * 1024
	}
	//并发量
	if fs.config.UploadConcurrency == 0 {
		fs.config.UploadConcurrency = s3manager.DefaultUploadConcurrency
	}
	//下载大小
	if fs.config.DownloadPartSize == 0 {
		fs.config.DownloadPartSize = s3manager.DefaultDownloadPartSize
	} else {
		fs.config.DownloadPartSize *= 1024 * 1024
	}
	//下载并发
	if fs.config.DownloadConcurrency == 0 {
		fs.config.DownloadConcurrency = s3manager.DefaultDownloadConcurrency
	}
	var accessSecret string
	if isValidated {
		accessSecret = fs.config.AccessSecret.GetPayload()
	} else {
		accessSecret = fs.config.AccessSecretPlaintext
	}
	fs.svc, err = oss.New(fs.config.Endpoint,
		fs.config.AccessKey,
		accessSecret,
	)
	if err != nil {
		return fs, err
	}
	return fs, nil
}
