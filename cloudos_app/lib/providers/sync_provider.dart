import 'dart:convert';
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

class CloudFile {
  final String path;
  final int size;
  final String status;
  final String hash;

  CloudFile({required this.path, required this.size, required this.status, required this.hash});

  factory CloudFile.fromJson(Map<String, dynamic> json) {
    return CloudFile(
      path: json['path'] ?? '',
      size: json['size'] ?? 0,
      status: json['status'] ?? 'active',
      hash: json['hash'] ?? '',
    );
  }
}

class SyncProvider with ChangeNotifier {
  bool isConnected = false;
  String targetIp = "";
  List<CloudFile> files = [];
  Timer? _syncTimer;

  void connect(String ip) {
    targetIp = ip;
    if (!targetIp.startsWith("http")) {
      targetIp = "http://$targetIp";
    }
    
    // Start polling the API
    _syncTimer?.cancel();
    _syncTimer = Timer.periodic(const Duration(seconds: 5), (_) => _syncFiles());
    _syncFiles(); // Initial sync
  }

  Future<void> _syncFiles() async {
    if (targetIp.isEmpty) return;

    try {
      final response = await http.get(Uri.parse('$targetIp/api/status')).timeout(const Duration(seconds: 3));
      if (response.statusCode == 200) {
        if (!isConnected) {
          isConnected = true;
          notifyListeners();
        }

        // Fetch files
        final filesResponse = await http.get(Uri.parse('$targetIp/api/files'));
        if (filesResponse.statusCode == 200) {
          final List<dynamic> data = json.decode(filesResponse.body);
          files = data.map((e) => CloudFile.fromJson(e)).toList();
          notifyListeners();
        }
      } else {
        _setDisconnected();
      }
    } catch (e) {
      _setDisconnected();
    }
  }

  void _setDisconnected() {
    if (isConnected) {
      isConnected = false;
      notifyListeners();
    }
  }

  @override
  void dispose() {
    _syncTimer?.cancel();
    super.dispose();
  }
}
