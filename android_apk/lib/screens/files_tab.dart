import 'dart:io';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:http/http.dart' as http;
import 'package:file_picker/file_picker.dart';
import 'package:path_provider/path_provider.dart';

import '../providers/sync_provider.dart';
import '../main.dart';

/// Files Tab = "EXPLORER" tab in the reference images.
/// Business logic (upload, download, http) is fully preserved.
class FilesTab extends StatefulWidget {
  const FilesTab({super.key});

  @override
  State<FilesTab> createState() => _FilesTabState();
}

class _FilesTabState extends State<FilesTab> {
  bool _isGrid = true;
  CloudFile? _selected;

  // ── unchanged business logic ──────────────────────────────────────────────
  Future<void> _uploadFile(BuildContext context, String targetIp) async {
    FilePickerResult? result = await FilePicker.platform.pickFiles();
    if (result != null) {
      File file = File(result.files.single.path!);
      var request = http.MultipartRequest('POST', Uri.parse('$targetIp/api/upload'));
      request.files.add(await http.MultipartFile.fromPath('file', file.path));
      var res = await request.send();
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(res.statusCode == 201 ? 'File uploaded!' : 'Upload failed.'),
          backgroundColor: res.statusCode == 201 ? AppColors.green : AppColors.red,
        ));
      }
    }
  }

  // ─────────────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final sync = context.watch<SyncProvider>();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // ── Page title row ────────────────────────────────────────
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 20, 16, 0),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              const Text('EXPLORER',
                style: TextStyle(fontSize: 28, fontWeight: FontWeight.w900,
                    color: AppColors.textPrimary, letterSpacing: 2)),
              const Spacer(),
              // NEW FILE button
              GestureDetector(
                onTap: () {
                  if (!sync.isConnected) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Connect to PC first!')));
                    return;
                  }
                  _uploadFile(context, sync.targetIp);
                },
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                  decoration: BoxDecoration(
                    border: Border.all(color: AppColors.border, width: 1.5),
                    color: AppColors.surface,
                  ),
                  child: const Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.add, size: 14, color: AppColors.textPrimary),
                      SizedBox(width: 6),
                      Text('NEW FILE', style: TextStyle(
                        fontSize: 10, fontWeight: FontWeight.w700, letterSpacing: 1.5)),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              // Refresh icon
              GestureDetector(
                onTap: () => sync.connect(sync.targetIp),
                child: Container(
                  width: 36, height: 36,
                  decoration: BoxDecoration(
                    color: AppColors.surface,
                    border: Border.all(color: AppColors.border),
                  ),
                  child: const Icon(Icons.refresh, size: 18, color: AppColors.textPrimary),
                ),
              ),
            ],
          ),
        ),

        // ── Breadcrumb ────────────────────────────────────────────
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
          child: Row(
            children: [
              const Text('ROOT', style: TextStyle(
                fontSize: 10, fontWeight: FontWeight.w700,
                color: AppColors.textSecondary, letterSpacing: 1.5)),
              const Icon(Icons.chevron_right, size: 14, color: AppColors.textMuted),
              Text(sync.isConnected ? 'VAULT' : 'LOCAL',
                style: const TextStyle(fontSize: 10, fontWeight: FontWeight.w700,
                    color: AppColors.yellow, letterSpacing: 1.5)),
            ],
          ),
        ),

        const SizedBox(height: 16),

        // ── File area + details panel ─────────────────────────────
        Expanded(
          child: sync.files.isEmpty
              ? _EmptyState(isConnected: sync.isConnected)
              : _FilesContent(
                  files: sync.files,
                  isGrid: _isGrid,
                  selected: _selected,
                  onSelect: (f) => setState(() => _selected = f),
                  onToggleView: () => setState(() => _isGrid = !_isGrid),
                ),
        ),
      ],
    );
  }
}

// ─── Widgets ──────────────────────────────────────────────────────────────────

class _EmptyState extends StatelessWidget {
  final bool isConnected;
  const _EmptyState({required this.isConnected});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.folder_open_outlined,
              size: 64, color: AppColors.textMuted),
            const SizedBox(height: 16),
            Text(
              isConnected
                  ? 'NO FILES SYNCED YET'
                  : 'NOT CONNECTED TO PC NODE',
              style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700,
                  letterSpacing: 2, color: AppColors.textSecondary),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              isConnected
                  ? 'Upload something to get started.'
                  : 'Go to MORE > Settings and enter your PC IP.',
              style: const TextStyle(fontSize: 11, color: AppColors.textMuted),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

class _FilesContent extends StatelessWidget {
  final List<CloudFile> files;
  final bool isGrid;
  final CloudFile? selected;
  final ValueChanged<CloudFile> onSelect;
  final VoidCallback onToggleView;

  const _FilesContent({
    required this.files,
    required this.isGrid,
    required this.selected,
    required this.onSelect,
    required this.onToggleView,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Tracked Files header with view toggle
        Container(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          decoration: BoxDecoration(
            color: AppColors.surface,
            border: Border.all(color: AppColors.border),
          ),
          child: Row(
            children: [
              const Text('TRACKED FILES', style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 2)),
              const Spacer(),
              // Grid / List toggle
              GestureDetector(
                onTap: onToggleView,
                child: Icon(
                  isGrid ? Icons.grid_view_rounded : Icons.view_list_outlined,
                  size: 18, color: AppColors.yellow),
              ),
              const SizedBox(width: 12),
              GestureDetector(
                onTap: onToggleView,
                child: Icon(
                  isGrid ? Icons.view_list_outlined : Icons.grid_view_rounded,
                  size: 18, color: AppColors.textMuted),
              ),
            ],
          ),
        ),
        const SizedBox(height: 1),

        Expanded(
          child: isGrid
              ? _GridView(files: files, selected: selected, onSelect: onSelect)
              : _ListView(files: files, selected: selected, onSelect: onSelect),
        ),

        // Details panel for selected file
        if (selected != null) _FileDetails(file: selected!),
      ],
    );
  }
}

class _GridView extends StatelessWidget {
  final List<CloudFile> files;
  final CloudFile? selected;
  final ValueChanged<CloudFile> onSelect;
  const _GridView({required this.files, required this.selected, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    return GridView.builder(
      padding: const EdgeInsets.all(16),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        mainAxisSpacing: 10,
        crossAxisSpacing: 10,
        childAspectRatio: 0.85,
      ),
      itemCount: files.length,
      itemBuilder: (_, i) {
        final f = files[i];
        final isSelected = selected?.path == f.path;
        final name = _filename(f);
        return GestureDetector(
          onTap: () => onSelect(f),
          child: Container(
            decoration: BoxDecoration(
              color: isSelected ? AppColors.yellowDim : AppColors.surface,
              border: Border.all(
                color: isSelected ? AppColors.yellow : AppColors.border,
                width: isSelected ? 1.5 : 1,
              ),
            ),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Stack(
                  clipBehavior: Clip.none,
                  children: [
                    Icon(_fileIcon(name), size: 36,
                        color: isSelected ? AppColors.yellow : AppColors.textSecondary),
                    if (f.status == 'syncing')
                      Positioned(
                        top: -4, right: -4,
                        child: Container(
                          width: 14, height: 14,
                          decoration: const BoxDecoration(
                            color: AppColors.blue, shape: BoxShape.circle),
                          child: const Icon(Icons.sync, size: 9, color: Colors.white),
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: 8),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 6),
                  child: Text(name,
                    style: const TextStyle(fontSize: 10, fontWeight: FontWeight.w600),
                    maxLines: 2, overflow: TextOverflow.ellipsis, textAlign: TextAlign.center),
                ),
                const SizedBox(height: 4),
                Text(_formatSize(f.size),
                  style: const TextStyle(fontSize: 9, color: AppColors.textMuted)),
              ],
            ),
          ),
        );
      },
    );
  }
}

class _ListView extends StatelessWidget {
  final List<CloudFile> files;
  final CloudFile? selected;
  final ValueChanged<CloudFile> onSelect;
  const _ListView({required this.files, required this.selected, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      itemCount: files.length,
      separatorBuilder: (_, __) => const SizedBox(height: 4),
      itemBuilder: (_, i) {
        final f = files[i];
        final isSelected = selected?.path == f.path;
        final name = _filename(f);
        return GestureDetector(
          onTap: () => onSelect(f),
          child: Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: isSelected ? AppColors.yellowDim : AppColors.surface,
              border: Border.all(
                  color: isSelected ? AppColors.yellow : AppColors.border,
                  width: isSelected ? 1.5 : 1),
            ),
            child: Row(
              children: [
                Icon(_fileIcon(name), size: 20,
                    color: isSelected ? AppColors.yellow : AppColors.textSecondary),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: const TextStyle(
                        fontSize: 12, fontWeight: FontWeight.w600)),
                      Text(_formatSize(f.size),
                        style: const TextStyle(fontSize: 10, color: AppColors.textMuted)),
                    ],
                  ),
                ),
                if (f.status == 'syncing')
                  const Icon(Icons.sync, size: 14, color: AppColors.blue),
                if (f.status == 'active')
                  const Icon(Icons.check_circle_outline, size: 14, color: AppColors.green),
              ],
            ),
          ),
        );
      },
    );
  }
}

class _FileDetails extends StatelessWidget {
  final CloudFile file;
  const _FileDetails({required this.file});

  @override
  Widget build(BuildContext context) {
    final name = _filename(file);
    return Container(
      decoration: const BoxDecoration(
        color: AppColors.surface,
        border: Border(top: BorderSide(color: AppColors.border)),
      ),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(name,
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
                  overflow: TextOverflow.ellipsis),
              ),
              if (file.status == 'syncing')
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  color: AppColors.blue,
                  child: const Text('SYNCING', style: TextStyle(
                    fontSize: 8, fontWeight: FontWeight.w700,
                    color: Colors.white, letterSpacing: 1)),
                ),
            ],
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              _DetailCol('SIZE', _formatSize(file.size)),
              _DetailCol('TYPE', _ext(name).toUpperCase()),
              _DetailCol('STATUS', file.status.toUpperCase()),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: ElevatedButton(
                  onPressed: () {},
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.textPrimary,
                    foregroundColor: Colors.black,
                    shape: const RoundedRectangleBorder(),
                    padding: const EdgeInsets.symmetric(vertical: 12),
                  ),
                  child: const Text('OPEN', style: TextStyle(
                    fontSize: 11, fontWeight: FontWeight.w700, letterSpacing: 2)),
                ),
              ),
              const SizedBox(width: 8),
              Container(
                width: 44, height: 44,
                color: AppColors.red.withOpacity(0.1),
                child: const Icon(Icons.delete_outline, color: AppColors.red, size: 20),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _DetailCol extends StatelessWidget {
  final String label;
  final String value;
  const _DetailCol(this.label, this.value);
  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: const TextStyle(
            fontSize: 8, fontWeight: FontWeight.w700,
            color: AppColors.textMuted, letterSpacing: 1.5)),
          const SizedBox(height: 2),
          Text(value, style: const TextStyle(
            fontSize: 11, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────
String _filename(CloudFile f) =>
    f.path.split('/').last.split('\\').last;

String _ext(String name) {
  final parts = name.split('.');
  return parts.length > 1 ? parts.last : 'FILE';
}

String _formatSize(int bytes) {
  if (bytes >= 1073741824) return '${(bytes / 1073741824).toStringAsFixed(1)} GB';
  if (bytes >= 1048576)    return '${(bytes / 1048576).toStringAsFixed(1)} MB';
  if (bytes >= 1024)       return '${(bytes / 1024).toStringAsFixed(1)} KB';
  return '$bytes B';
}

IconData _fileIcon(String name) {
  final ext = name.split('.').last.toLowerCase();
  switch (ext) {
    case 'xlsx': case 'xls': case 'csv': return Icons.table_chart_outlined;
    case 'pdf':  return Icons.picture_as_pdf_outlined;
    case 'zip':  case 'tar': case 'gz': return Icons.archive_outlined;
    case 'sh':   case 'py':  case 'js': case 'ts': case 'go': case 'dart':
      return Icons.terminal_outlined;
    case 'png':  case 'jpg': case 'jpeg': case 'webp':
      return Icons.image_outlined;
    case 'mp4':  case 'mov': return Icons.video_file_outlined;
    default: return Icons.insert_drive_file_outlined;
  }
}
