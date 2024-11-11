""" Without error bars"""

import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

# 读取两个 CSV 数据，分别为 Channel 版本和 Memory Sharing 版本。
channel_data = pd.read_csv('channel_version.csv', header=0, names=['name', 'time', 'range'])
memory_sharing_data = pd.read_csv('memory_sharing_version.csv', header=0, names=['name', 'time', 'range'])

# 添加 'group' 列，以区分 Channel 和 Memory Sharing 版本的数据。
channel_data['group'] = 'Channel'
memory_sharing_data['group'] = 'Memory Sharing'

# 合并两个数据集
benchmark_data = pd.concat([channel_data, memory_sharing_data], ignore_index=True)

# 将 'time' 列的数据类型转换为浮点数。
benchmark_data['time'] = benchmark_data['time'].astype(float)

# 从 'range' 列中去掉百分号并转换为浮点数。
benchmark_data['range'] = benchmark_data['range'].str.rstrip('%').astype(float)

# 移除 'name' 列中包含 NaN 的行。
benchmark_data = benchmark_data.dropna(subset=['name'])

# 仅保留 'name' 列中包含特定模式的行。
benchmark_data = benchmark_data[benchmark_data['name'].str.contains(r'Gol/512x512x1000-\d+-\d+')]

# 从基准测试名称中提取使用的线程数。
benchmark_data['threads'] = benchmark_data['name'].str.extract(r'Gol/512x512x1000-(\d+)-\d+')[0].astype(int)

# 计算误差值，根据百分比计算绝对误差。
benchmark_data['error'] = benchmark_data['time'] * benchmark_data['range'] / 100

# 打印数据以检查是否正确解析。
print(benchmark_data)

# 绘制带有误差条的折线图并对比两组数据。
plt.figure(figsize=(10, 6))
ax = sns.lineplot(data=benchmark_data, x='threads', y='time', hue='group', marker='o')

# 添加误差条。
for group in benchmark_data['group'].unique():
    group_data = benchmark_data[benchmark_data['group'] == group]
    plt.errorbar(group_data['threads'], group_data['time'], yerr=group_data['error'], fmt='none', label=f"{group} Error", capsize=5)

# 设置坐标轴标签和标题。
ax.set_xlabel('Number of Threads Used')
ax.set_ylabel('Time Taken (seconds)')
ax.set_title('Game of Life Benchmark Results: Threads vs Time (Channel vs Memory Sharing)')

# 添加网格以提高可读性。
plt.grid(True)

# 显示完整的图形。
plt.show()


# """ With error bars"""
# import pandas as pd
# import matplotlib.pyplot as plt
# import seaborn as sns

# # 读取保存的 CSV 数据。
# benchmark_data = pd.read_csv('results.csv', header=0, names=['name', 'time', 'range'])

# # 将 'time' 列的数据类型转换为浮点数。
# benchmark_data['time'] = benchmark_data['time'].astype(float)

# # 从 'range' 列中去掉百分号并转换为浮点数。
# benchmark_data['range'] = benchmark_data['range'].str.rstrip('%').astype(float)

# # 移除 'name' 列中包含 NaN 的行（例如空行或格式不正确的行）。
# benchmark_data = benchmark_data.dropna(subset=['name'])

# # 仅保留 'name' 列中包含特定模式的行，防止不匹配的行造成问题。
# benchmark_data = benchmark_data[benchmark_data['name'].str.contains(r'Gol/512x512x1000-\d+-\d+')]

# # 从基准测试名称中提取使用的线程数。
# benchmark_data['threads'] = benchmark_data['name'].str.extract(r'Gol/512x512x1000-(\d+)-\d+')[0].astype(int)

# # 计算误差值，根据百分比计算绝对误差。
# benchmark_data['error'] = benchmark_data['time'] * benchmark_data['range'] / 100

# # 打印数据以检查是否正确解析。
# print(benchmark_data)

# # 绘制带有误差条的折线图。
# plt.figure(figsize=(10, 6))
# ax = sns.lineplot(data=benchmark_data, x='threads', y='time', marker='o', label='Average Time')

# # 添加误差条。
# plt.errorbar(benchmark_data['threads'], benchmark_data['time'], yerr=benchmark_data['error'], fmt='none', c='blue', capsize=5)

# # 设置坐标轴标签和标题。
# ax.set_xlabel('Number of Threads Used')
# ax.set_ylabel('Time Taken (seconds)')
# ax.set_title('Game of Life Benchmark Results: Threads vs Time (with Error Bars)')

# # 添加网格以提高可读性。
# plt.grid(True)

# # 显示完整的图形。
# plt.show()