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

class HeaderController {

  constructor(mainView, componentRegistry) {
    this.headerView = new HeaderView();
    this.headerView.onPipelineTypeChanged(
      pipelineType => this.pipelineTypeChanged(pipelineType)
    );
    mainView.appendView(this.headerView);
    this.componentListeners = [];
    this.componentRegistry = componentRegistry;
  }

  addComponentListener(componentListener) {
    this.componentListeners.push(componentListener);
  }

  pipelineTypeChanged(pipelineType) {
    this.pipelineType = pipelineType;
    this.componentListeners.forEach(l => l.pipelineTypeSelected(pipelineType));
  }

  getPipelineType() {
    return this.pipelineType;
  }

  fetchComponents() {
    fetch('http://localhost:8888/components').then(
      resp => resp.json().then(
        components => {
          this.headerView.enableSelection();
          this.componentRegistry.setComponents(components);
        }
      )
    );
  }

}
