// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

window.onload = main;

function main() {
  let componentRegistry = new ComponentRegistry();
  let mainView = new View();
  document.body.appendChild(mainView.getRootEl());
  let headerController = new HeaderController(mainView, componentRegistry);
  headerController.fetchComponents();
  let receiverController = new ComponentController('receiver', mainView, componentRegistry);
  let processorController = new ComponentController('processor', mainView, componentRegistry);
  let exporterController = new ComponentController('exporter', mainView, componentRegistry);

  let exportPanelController = new ExportPanelController(mainView);
  exportPanelController.setPipelineTypeProvider(headerController);
  exportPanelController.setReceiverController(receiverController);
  exportPanelController.setProcessorController(processorController);
  exportPanelController.setExporterController(exporterController);

  headerController.addComponentListener(receiverController);
  headerController.addComponentListener(processorController);
  headerController.addComponentListener(exporterController);
  headerController.addComponentListener(exportPanelController);
}
