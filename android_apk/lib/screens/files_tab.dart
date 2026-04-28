import 'dart:io';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:http/http.dart' as http;
import 'package:file_picker/file_picker.dart';
import 'package:path_provider/path_provider.dart';

import '../providers/sync_provider.dart';

class FilesTab extends StatelessWidget {
  const FilesTab({super.key});

  Future<void> _uploadFile(BuildContext context, String targetIp) async {
    FilePickerResult? result = await FilePicker.platform.pickFiles();
    if (result != null) {
      File file = File(result.files.single.path!);
      
      var request = http.MultipartRequest('POST', Uri.parse('$targetIp/api/upload'));
      request.files.add(await http.MultipartFile.fromPath('file', file.path));

      var res = await request.send();
      if (res.statusCode == 201) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('File uploaded successfully!')));
      } else {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Failed to upload.')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final syncProvider = context.watch<SyncProvider>();

    return Scaffold(
      body: syncProvider.files.isEmpty
          ? const Center(child: Text("No files synced yet. Upload something!"))
          : ListView.builder(
              padding: const EdgeInsets.all(8.0),
              itemCount: syncProvider.files.length,
              itemBuilder: (context, index) {
                final file = syncProvider.files[index];
                final filename = file.path.split('/').last.split('\\').last;

                return Card(
                  margin: const EdgeInsets.symmetric(vertical: 6.0, horizontal: 8.0),
                  child: ListTile(
                    leading: const Icon(Icons.insert_drive_file, color: Colors.blueAccent),
                    title: Text(filename, style: const TextStyle(fontWeight: FontWeight.bold)),
                    subtitle: Text('${(file.size / 1024).toStringAsFixed(1)} KB'),
                    trailing: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        IconButton(
                          icon: const Icon(Icons.download),
                          onPressed: () {
                            ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Downloading $filename...')));
                            // Future enhancement: Add physical download via REST or P2P logic here
                          },
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () {
          if (!syncProvider.isConnected) {
            ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Connect to PC first!')));
            return;
          }
          _uploadFile(context, syncProvider.targetIp);
        },
        icon: const Icon(Icons.cloud_upload),
        label: const Text('Upload'),
      ),
    );
  }
}
