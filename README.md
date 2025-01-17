# MyGlance: 基于Glance的二次开发

**原项目地址：**[glanceapp/glance](https://github.com/glanceapp/glance)

我很喜欢glance的设计，但是对我来它还存在一定的改良空间，所以我基于glance进行了二次开发，这里是我的一些记录。
因为基本是面向我个人的所以现在说明还很不完善QAQ

## 现在完成了什么？

**2024-0628更新：**

- [x] 支持bilibili视频订阅（参考了[https://github.com/glanceapp/glance/pull/100]的部分代码）
- [x] 上传了docker镜像
- [x] 添加了docker-compose构建文件

**2024-0625更新：**

- [x] 添加了中文字体（通过第三方CDN）
- [x] 添加了http代理设置，可用于解锁反爬机制严格的网站或者部署在国内使用（在配置文件server中添加proxy-url即可）
- [x] 我在意的一些细节

## 部署方式

### 使用docker-compose(推荐)

```shell
git clone https://github.com/7Sageer/MyGlance.git
cd MyGlance
vi glance.yml # 修改配置文件
docker-compose up -d
```

### 使用docker镜像

```shell
docker run -d -p 8080:8080 -v /path/to/glance.yml:/app/glance.yml --name myglance qihr2022/glance
```

## 未来我想做什么？

- [ ] 细化代理设置
- [ ] 更好地支持中文社区订阅
- [ ] 更方便的部署方式
- [ ] 对于对于新增的配置项，添加更详细的说明
- [ ] 更好的主题自定义
- [ ] 其他我在意的需求
